package service

import (
	"context"
	"fmt"
	"sync"
	"time"
	"encoding/json"
	"strings"

	"cis/pkg/types"
	"cis/pkg/registry"
	"github.com/robfig/cron"
	"cis/pkg/db"
	"cis/pkg/models"
	"gorm.io/gorm"
)

type workflowServiceImpl struct {
	sync.RWMutex
	tasks     map[string]context.CancelFunc
	scheduler *time.Ticker
	stopChan  chan struct{}
}

func NewWorkflowService() WorkflowService {
	svc := &workflowServiceImpl{
		tasks:    make(map[string]context.CancelFunc),
		scheduler: time.NewTicker(1 * time.Minute),  // 每分钟检查一次
		stopChan:  make(chan struct{}),
	}

	// 启动调度器
	go svc.runScheduler()

	// 恢复异常终止的工作流状态
	go svc.recoverWorkflowStates()

	return svc
}

// runScheduler 运行定时调度器
func (w *workflowServiceImpl) runScheduler() {
	for {
		select {
		case <-w.scheduler.C:
			w.checkScheduledWorkflows()
		case <-w.stopChan:
			w.scheduler.Stop()
			return
		}
	}
}

// checkScheduledWorkflows 检查并执行到期的定时工作流
func (w *workflowServiceImpl) checkScheduledWorkflows() {
	// 从数据库获取所有工作流
	var dbWorkflows []models.Workflow
	if err := db.DB.Find(&dbWorkflows).Error; err != nil {
		fmt.Printf("获取工作流列表失败: %v\n", err)
		return
	}

	now := time.Now()
	for _, dbWf := range dbWorkflows {
		// 转换为 WorkflowSpec
		wf, err := dbWf.ToSpec()
		if err != nil {
			fmt.Printf("转换工作流失败: %v\n", err)
			continue
		}

		if wf.Schedule != nil && wf.Schedule.Enabled {
			if now.After(wf.Schedule.NextRun) {
				// 执行工作流
				go func(name string) {
					if err := w.ExecuteWorkflow(context.Background(), name); err != nil {
						fmt.Printf("定时执行工作流 %s 失败: %v\n", name, err)
					}
				}(wf.Name)

				// 计算下次执行时间
				if schedule, err := cron.Parse(wf.Schedule.Cron); err == nil {
					wf.Schedule.NextRun = schedule.Next(now)
					// 更新数据库中的调度时间
					updatedSchedule, _ := json.Marshal(wf.Schedule)
					if err := db.DB.Model(&models.Workflow{}).
						Where("name = ?", wf.Name).
						Update("schedule", string(updatedSchedule)).Error; err != nil {
						fmt.Printf("更新工作流调度时间失败: %v\n", err)
					}
				}
			}
		}
	}
}

func (w *workflowServiceImpl) CreateWorkflow(ctx context.Context, spec *types.WorkflowSpec) error {
	userID, ok := ctx.Value("user_id").(uint)
	if !ok {
		return fmt.Errorf("未授权的访问")
	}
	
	// 检查是否存在
	var count int64
	if err := db.DB.Model(&models.Workflow{}).Where("name = ? AND user_id = ?", spec.Name, userID).Count(&count).Error; err != nil {
		return fmt.Errorf("检查工作流是否存在时发生错误: %v", err)
	}
	if count > 0 {
		return fmt.Errorf("工作流 '%s' 已存在，请使用其他名称", spec.Name)
	}

	// 设置初始状态和时间
	now := time.Now()
	spec.Status = types.WorkflowStatusPending
	spec.CreateTime = now
	spec.UpdateTime = now
	spec.Logs = make([]types.WorkflowLog, 0)

	// 如果配置了定时任务，计算下次执行时间
	if spec.Schedule != nil && spec.Schedule.Enabled {
		if schedule, err := cron.Parse(spec.Schedule.Cron); err == nil {
			spec.Schedule.NextRun = schedule.Next(now)
		} else {
			return fmt.Errorf("无效的 Cron 表达式: %v", err)
		}
	}

	// 创建数据库记录
	workflow := &models.Workflow{}
	workflow.Name = spec.Name
	workflow.UserID = userID
	workflow.Status = string(spec.Status)

	// 初始化空日志数组
	emptyLogs, _ := json.Marshal(make([]types.WorkflowLog, 0))
	workflow.Logs = string(emptyLogs)

	// 序列化各个字段
	if source, err := json.Marshal(spec.Source); err == nil {
		workflow.Source = string(source)
	}
	if targets, err := json.Marshal(spec.Targets); err == nil {
		workflow.Targets = string(targets)
	}
	if images, err := json.Marshal(spec.Images); err == nil {
		workflow.Images = string(images)
	}
	if spec.Schedule != nil {
		if schedule, err := json.Marshal(spec.Schedule); err == nil {
			workflow.Schedule = string(schedule)
		}
	}

	if err := db.DB.Create(workflow).Error; err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return fmt.Errorf("工作流 '%s' 已存在，请使用其他名称", spec.Name)
		}
		return fmt.Errorf("创建工作流记录失败: %v", err)
	}

	// 设置 spec 的 UserID，这样 addWorkflowLog 可以正确工作
	spec.UserID = userID

	// 添加创建日志
	w.addWorkflowLog(spec, "info", "工作流创建成功")
	return nil
}

func (w *workflowServiceImpl) ListWorkflows(ctx context.Context) ([]*types.WorkflowSpec, error) {
	userID, ok := ctx.Value("user_id").(uint)
	if !ok {
		return nil, fmt.Errorf("未授权的访问")
	}
	var workflows []models.Workflow
	if err := db.DB.Where("user_id = ?", userID).Find(&workflows).Error; err != nil {
		return nil, err
	}

	specs := make([]*types.WorkflowSpec, 0, len(workflows))
	for _, workflow := range workflows {
		spec, err := workflow.ToSpec()
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func (w *workflowServiceImpl) GetWorkflow(ctx context.Context, name string) (*types.WorkflowSpec, error) {
	userID, ok := ctx.Value("user_id").(uint)
	if !ok {
		return nil, fmt.Errorf("未授权的访问")
	}

	var workflow models.Workflow
	if err := db.DB.Where("name = ? AND user_id = ?", name, userID).First(&workflow).Error; err != nil {
		return nil, fmt.Errorf("工作流 %s 不存在", name)
	}

	return workflow.ToSpec()
}

func (w *workflowServiceImpl) UpdateWorkflow(ctx context.Context, spec *types.WorkflowSpec) error {
	userID := ctx.Value("user_id").(uint)
	// 确保设置了 UserID
	spec.UserID = userID

	// 先获取现有的工作流
	var workflow models.Workflow
	if err := db.DB.Where("name = ? AND user_id = ?", spec.Name, userID).First(&workflow).Error; err != nil {
		return fmt.Errorf("工作流 %s 不存在", spec.Name)
	}

	// 获取现有的工作流规范，保留状态和运行记录
	existingSpec, err := workflow.ToSpec()
	if err != nil {
		return fmt.Errorf("解析工作流数据失败: %v", err)
	}

	// 保留原有的状态相关字段
	spec.Status = existingSpec.Status
	spec.LastRun = existingSpec.LastRun
	spec.Logs = existingSpec.Logs
	spec.CreateTime = existingSpec.CreateTime

	// 更新时间
	spec.UpdateTime = time.Now()

	// 更新数据库记录
	if err := workflow.FromSpec(spec); err != nil {
		return err
	}

	if err := db.DB.Save(&workflow).Error; err != nil {
		return err
	}

	// 添加更新日志
	w.addWorkflowLog(spec, "info", "工作流配置已更新")
	return nil
}

func (w *workflowServiceImpl) DeleteWorkflow(ctx context.Context, name string) error {
	userID := ctx.Value("user_id").(uint)
	if err := db.DB.Where("name = ? AND user_id = ?", name, userID).Delete(&models.Workflow{}).Error; err != nil {
		return err
	}
	return nil
}

func (w *workflowServiceImpl) ExecuteWorkflow(ctx context.Context, name string) error {
	userID := ctx.Value("user_id").(uint)
	workflow, err := w.GetWorkflow(ctx, name)
	if err != nil {
		return err
	}

	w.Lock()
	defer w.Unlock()

	// 检查工作流状态，如果是 running 状态但没有对应的任务，说明是异常终止
	if workflow.Status == types.WorkflowStatusRunning {
		if _, exists := w.tasks[name]; !exists {
			// 重置状态
			workflow.Status = types.WorkflowStatusFailed
			run := &types.WorkflowRun{
				StartTime: workflow.LastRun.StartTime,
				EndTime:   time.Now(),
				Status:    types.WorkflowStatusFailed,
				Message:   "工作流异常终止",
			}
			workflow.LastRun = run

			// 更新数据库
			var dbWorkflow models.Workflow
			if err := db.DB.Where("name = ? AND user_id = ?", name, userID).First(&dbWorkflow).Error; err == nil {
				lastRun, _ := json.Marshal(run)
				dbWorkflow.Status = string(workflow.Status)
				dbWorkflow.LastRun = string(lastRun)
				db.DB.Save(&dbWorkflow)
			}
		}
	}

	// 检查工作流是否正在运行
	if workflow.Status == types.WorkflowStatusRunning {
		return fmt.Errorf("工作流 %s 正在运行中", name)
	}

	// 确保设置了 UserID
	workflow.UserID = userID

	// 创建新的上下文用于任务取消
	taskCtx, cancel := context.WithCancel(context.Background())
	w.tasks[name] = cancel

	// 更新工作流状态
	run := &types.WorkflowRun{
		StartTime: time.Now(),
		Status:    types.WorkflowStatusRunning,
	}
	
	workflow.Status = types.WorkflowStatusRunning
	workflow.LastRun = run
	
	// 更新数据库中的状态
	var dbWorkflow models.Workflow
	if err := db.DB.Where("name = ? AND user_id = ?", name, userID).First(&dbWorkflow).Error; err != nil {
		return fmt.Errorf("获取工作流失败: %v", err)
	}
	
	// 更新状态和 LastRun
	lastRun, _ := json.Marshal(run)
	dbWorkflow.Status = string(workflow.Status)
	dbWorkflow.LastRun = string(lastRun)
	
	if err := db.DB.Save(&dbWorkflow).Error; err != nil {
		return fmt.Errorf("保存工作流状态失败: %v", err)
	}

	w.addWorkflowLog(workflow, "info", fmt.Sprintf("开始执行工作流，包含 %d 个镜像，目标仓库数量: %d", 
		len(workflow.Images), len(workflow.Targets)))

	// 在后台执行同步任务
	go func() {
		defer func() {
			w.Lock()
			delete(w.tasks, name)
			w.Unlock()
		}()

		syncer := registry.NewImageSyncer(workflow.Source, workflow.Targets)
		var syncErrors []string
		
		for _, image := range workflow.Images {
			select {
			case <-taskCtx.Done():
				w.Lock()
				run.EndTime = time.Now()
				run.Status = types.WorkflowStatusFailed
				run.Message = "工作流被取消"
				workflow.Status = types.WorkflowStatusFailed
				w.addWorkflowLog(workflow, "warn", "工作流被取消")
				
				// 更新数据库
				var dbWorkflow models.Workflow
				if err := db.DB.Where("name = ? AND user_id = ?", workflow.Name, workflow.UserID).First(&dbWorkflow).Error; err == nil {
					lastRun, _ := json.Marshal(run)
					dbWorkflow.Status = string(workflow.Status)
					dbWorkflow.LastRun = string(lastRun)
					db.DB.Save(&dbWorkflow)
				}
				w.Unlock()
				return
			default:
				w.Lock()
				w.addWorkflowLog(workflow, "info", fmt.Sprintf("开始同步镜像: %s", image))
				w.Unlock()

				if err := syncer.SyncImage(taskCtx, image); err != nil {
					w.Lock()
					syncErrors = append(syncErrors, fmt.Sprintf("同步镜像 %s 失败: %v", image, err))
					w.addWorkflowLog(workflow, "error", fmt.Sprintf("同步镜像 %s 失败: %v", image, err))
					w.Unlock()
					continue
				}

				w.Lock()
				w.addWorkflowLog(workflow, "info", fmt.Sprintf("镜像 %s 同步成功", image))
				w.Unlock()
			}
		}

		w.Lock()
		run.EndTime = time.Now()
		if len(syncErrors) > 0 {
			run.Status = types.WorkflowStatusFailed
			run.Message = fmt.Sprintf("部分镜像同步失败：\n%s", strings.Join(syncErrors, "\n"))
		} else {
			run.Status = types.WorkflowStatusSuccess
			run.Message = fmt.Sprintf("成功同步 %d 个镜像到 %d 个目标仓库", len(workflow.Images), len(workflow.Targets))
		}
		workflow.Status = run.Status
		w.addWorkflowLog(workflow, "info", run.Message)
		
		// 更新数据库
		var dbWorkflow models.Workflow
		if err := db.DB.Where("name = ? AND user_id = ?", workflow.Name, workflow.UserID).First(&dbWorkflow).Error; err == nil {
			lastRun, _ := json.Marshal(run)
			dbWorkflow.Status = string(workflow.Status)
			dbWorkflow.LastRun = string(lastRun)
			db.DB.Save(&dbWorkflow)
		}
		w.Unlock()
	}()

	return nil
}

// CancelWorkflow 取消正在执行的工作流
func (w *workflowServiceImpl) CancelWorkflow(ctx context.Context, name string) error {
	w.Lock()
	defer w.Unlock()

	cancel, exists := w.tasks[name]
	if !exists {
		return fmt.Errorf("工作流 %s 未在运行", name)
	}

	cancel()
	return nil
}

// addWorkflowLog 添加工作流日志
func (w *workflowServiceImpl) addWorkflowLog(workflow *types.WorkflowSpec, level, message string) {
	userID := workflow.UserID
	log := types.WorkflowLog{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}

	// 保存到数据库
	var dbWorkflow models.Workflow
	if err := db.DB.Where("name = ? AND user_id = ?", workflow.Name, userID).First(&dbWorkflow).Error; err != nil {
		// 如果记录不存在，可能是正在创建过程中
		fmt.Printf("工作流 %s 的日志将在创建完成后保存\n", workflow.Name)
		return
	}

	// 获取现有日志
	var existingLogs []types.WorkflowLog
	if dbWorkflow.Logs != "" {
		if err := json.Unmarshal([]byte(dbWorkflow.Logs), &existingLogs); err != nil {
			fmt.Printf("解析现有日志失败: %v，将创建新的日志列表\n", err)
			existingLogs = make([]types.WorkflowLog, 0)
		}
	} else {
		existingLogs = make([]types.WorkflowLog, 0)
	}

	// 添加新日志
	existingLogs = append(existingLogs, log)
	workflow.Logs = existingLogs

	// 序列化日志
	logsJSON, err := json.Marshal(existingLogs)
	if err != nil {
		fmt.Printf("序列化日志失败: %v\n", err)
		return
	}

	// 使用事务更新数据库
	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		// 重新获取最新数据
		if err := tx.Where("name = ? AND user_id = ?", workflow.Name, userID).First(&dbWorkflow).Error; err != nil {
			return err
		}
		
		// 更新日志字段
		if err := tx.Model(&dbWorkflow).Update("logs", string(logsJSON)).Error; err != nil {
			return err
		}
		
		return nil
	}); err != nil {
		fmt.Printf("保存日志失败: %v\n", err)
		return
	}
}

// recoverWorkflowStates 恢复异常终止的工作流状态
func (w *workflowServiceImpl) recoverWorkflowStates() {
	// 查找所有处于 running 状态的工作流
	var workflows []models.Workflow
	if err := db.DB.Where("status = ?", string(types.WorkflowStatusRunning)).Find(&workflows).Error; err != nil {
		fmt.Printf("查询运行中的工作流失败: %v\n", err)
		return
	}

	for _, workflow := range workflows {
		// 将状态更新为失败
		run := &types.WorkflowRun{
			StartTime: workflow.UpdatedAt,
			EndTime:   time.Now(),
			Status:    types.WorkflowStatusFailed,
			Message:   "工作流异常终止，系统重启后自动恢复状态",
		}

		lastRun, _ := json.Marshal(run)
		
		// 使用事务更新状态和添加日志
		if err := db.DB.Transaction(func(tx *gorm.DB) error {
			// 更新状态和最后运行记录
			if err := tx.Model(&workflow).Updates(map[string]interface{}{
				"status":   string(types.WorkflowStatusFailed),
				"last_run": string(lastRun),
			}).Error; err != nil {
				return err
			}

			// 添加日志
			var logs []types.WorkflowLog
			if workflow.Logs != "" {
				if err := json.Unmarshal([]byte(workflow.Logs), &logs); err == nil {
					logs = append(logs, types.WorkflowLog{
						Timestamp: time.Now(),
						Level:     "error",
						Message:   "工作流异常终止，系统重启后自动恢复状态",
					})
					if logsJSON, err := json.Marshal(logs); err == nil {
						if err := tx.Model(&workflow).Update("logs", string(logsJSON)).Error; err != nil {
							return err
						}
					}
				}
			}

			return nil
		}); err != nil {
			fmt.Printf("恢复工作流 %s 状态失败: %v\n", workflow.Name, err)
		} else {
			fmt.Printf("已恢复工作流 %s 的状态\n", workflow.Name)
		}
	}
} 