package service

import (
	"context"
	"fmt"
	"sync"
	"time"
	"encoding/json"

	"cis/pkg/types"
	"cis/pkg/registry"
	"github.com/robfig/cron"
	"cis/pkg/db"
	"cis/pkg/models"
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
	// 检查是否存在
	var count int64
	if err := db.DB.Model(&models.Workflow{}).Where("name = ?", spec.Name).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("工作流 %s 已存在", spec.Name)
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
	// 先设置基本字段
	workflow.Name = spec.Name
	workflow.Status = string(spec.Status)

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
	if logs, err := json.Marshal(spec.Logs); err == nil {
		workflow.Logs = string(logs)
	}
	if spec.Schedule != nil {
		if schedule, err := json.Marshal(spec.Schedule); err == nil {
			workflow.Schedule = string(schedule)
		}
	}

	if err := db.DB.Create(workflow).Error; err != nil {
		return fmt.Errorf("创建工作流记录失败: %v", err)
	}

	// 添加创建日志
	w.addWorkflowLog(spec, "info", "工作流创建成功")
	return nil
}

func (w *workflowServiceImpl) ListWorkflows(ctx context.Context) ([]*types.WorkflowSpec, error) {
	var workflows []models.Workflow
	if err := db.DB.Find(&workflows).Error; err != nil {
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
	var workflow models.Workflow
	if err := db.DB.Where("name = ?", name).First(&workflow).Error; err != nil {
		return nil, fmt.Errorf("工作流 %s 不存在", name)
	}

	return workflow.ToSpec()
}

func (w *workflowServiceImpl) UpdateWorkflow(ctx context.Context, spec *types.WorkflowSpec) error {
	var workflow models.Workflow
	if err := db.DB.Where("name = ?", spec.Name).First(&workflow).Error; err != nil {
		return fmt.Errorf("工作流 %s 不存在", spec.Name)
	}

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
	if err := db.DB.Where("name = ?", name).Delete(&models.Workflow{}).Error; err != nil {
		return err
	}
	return nil
}

func (w *workflowServiceImpl) ExecuteWorkflow(ctx context.Context, name string) error {
	workflow, err := w.GetWorkflow(ctx, name)
	if err != nil {
		return err
	}

	w.Lock()
	// 检查工作流是否正在运行
	if workflow.Status == types.WorkflowStatusRunning {
		w.Unlock()
		return fmt.Errorf("工作流 %s 正在运行中", name)
	}

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
	if err := db.DB.Where("name = ?", name).First(&dbWorkflow).Error; err != nil {
		w.Unlock()
		return fmt.Errorf("获取工作流失败: %v", err)
	}
	
	// 更新状态和 LastRun
	lastRun, _ := json.Marshal(run)
	dbWorkflow.Status = string(workflow.Status)
	dbWorkflow.LastRun = string(lastRun)
	
	if err := db.DB.Save(&dbWorkflow).Error; err != nil {
		w.Unlock()
		return fmt.Errorf("保存工作流状态失败: %v", err)
	}

	w.addWorkflowLog(workflow, "info", fmt.Sprintf("开始执行工作流，包含 %d 个镜像，目标仓库数量: %d", 
		len(workflow.Images), len(workflow.Targets)))
	w.Unlock()

	// 在后台执行同步任务
	go func() {
		defer func() {
			w.Lock()
			delete(w.tasks, name)
			w.Unlock()
		}()

		syncer := registry.NewImageSyncer(workflow.Source, workflow.Targets)
		
		for _, image := range workflow.Images {
			// 检查任务是否被取消
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
				if err := db.DB.Where("name = ?", workflow.Name).First(&dbWorkflow).Error; err == nil {
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
					run.EndTime = time.Now()
					run.Status = types.WorkflowStatusFailed
					run.Message = fmt.Sprintf("同步镜像 %s 失败: %v", image, err)
					workflow.Status = types.WorkflowStatusFailed
					w.addWorkflowLog(workflow, "error", run.Message)
					
					// 更新数据库
					var dbWorkflow models.Workflow
					if err := db.DB.Where("name = ?", workflow.Name).First(&dbWorkflow).Error; err == nil {
						lastRun, _ := json.Marshal(run)
						dbWorkflow.Status = string(workflow.Status)
						dbWorkflow.LastRun = string(lastRun)
						db.DB.Save(&dbWorkflow)
					}
					w.Unlock()
					return
				}

				w.Lock()
				w.addWorkflowLog(workflow, "info", fmt.Sprintf("镜像 %s 同步成功", image))
				w.Unlock()
			}
		}

		w.Lock()
		run.EndTime = time.Now()
		run.Status = types.WorkflowStatusSuccess
		run.Message = fmt.Sprintf("成功同步 %d 个镜像到 %d 个目标仓库", len(workflow.Images), len(workflow.Targets))
		workflow.Status = types.WorkflowStatusSuccess
		w.addWorkflowLog(workflow, "info", run.Message)
		
		// 更新数据库
		var dbWorkflow models.Workflow
		if err := db.DB.Where("name = ?", workflow.Name).First(&dbWorkflow).Error; err == nil {
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
	log := types.WorkflowLog{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}
	workflow.Logs = append(workflow.Logs, log)

	// 保存到数据库
	var dbWorkflow models.Workflow
	if err := db.DB.Where("name = ?", workflow.Name).First(&dbWorkflow).Error; err != nil {
		// 如果记录不存在，可能是正在创建过程中
		fmt.Printf("工作流 %s 的日志将在创建完成后保存\n", workflow.Name)
		return
	}

	// 获取现有日志
	var existingLogs []types.WorkflowLog
	if dbWorkflow.Logs != "" {
		if err := json.Unmarshal([]byte(dbWorkflow.Logs), &existingLogs); err == nil {
			workflow.Logs = append(existingLogs, log)
		}
	}

	// 序列化日志
	logs, err := json.Marshal(workflow.Logs)
	if err != nil {
		fmt.Printf("序列化日志失败: %v\n", err)
		return
	}

	// 更新数据库
	if err := db.DB.Model(&dbWorkflow).Update("logs", string(logs)).Error; err != nil {
		fmt.Printf("保存日志失败: %v\n", err)
		return
	}
} 