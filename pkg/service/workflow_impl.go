package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cis/pkg/types"
	"cis/pkg/registry"
	"github.com/robfig/cron"
)

type workflowServiceImpl struct {
	sync.RWMutex
	workflows map[string]*types.WorkflowSpec
	tasks    map[string]context.CancelFunc // 用于存储和管理正在运行的任务
	scheduler *time.Ticker
	stopChan  chan struct{}
}

func NewWorkflowService() WorkflowService {
	svc := &workflowServiceImpl{
		workflows: make(map[string]*types.WorkflowSpec),
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
	w.RLock()
	workflows := make([]*types.WorkflowSpec, 0, len(w.workflows))
	for _, wf := range w.workflows {
		workflows = append(workflows, wf)
	}
	w.RUnlock()

	now := time.Now()
	for _, wf := range workflows {
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
					w.Lock()
					wf.Schedule.NextRun = schedule.Next(now)
					w.Unlock()
				}
			}
		}
	}
}

func (w *workflowServiceImpl) CreateWorkflow(ctx context.Context, spec *types.WorkflowSpec) error {
	w.Lock()
	defer w.Unlock()

	if _, exists := w.workflows[spec.Name]; exists {
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

	w.workflows[spec.Name] = spec
	
	// 添加创建日志
	w.addWorkflowLog(spec, "info", "工作流创建成功")
	return nil
}

func (w *workflowServiceImpl) ListWorkflows(ctx context.Context) ([]*types.WorkflowSpec, error) {
	w.RLock()
	defer w.RUnlock()

	workflows := make([]*types.WorkflowSpec, 0, len(w.workflows))
	for _, workflow := range w.workflows {
		workflows = append(workflows, workflow)
	}
	return workflows, nil
}

func (w *workflowServiceImpl) GetWorkflow(ctx context.Context, name string) (*types.WorkflowSpec, error) {
	w.RLock()
	defer w.RUnlock()

	workflow, exists := w.workflows[name]
	if !exists {
		return nil, fmt.Errorf("工作流 %s 不存在", name)
	}
	return workflow, nil
}

func (w *workflowServiceImpl) UpdateWorkflow(ctx context.Context, spec *types.WorkflowSpec) error {
	w.Lock()
	defer w.Unlock()

	existing, exists := w.workflows[spec.Name]
	if !exists {
		return fmt.Errorf("工作流 %s 不存在", spec.Name)
	}

	// 保持原有的一些字段
	spec.CreateTime = existing.CreateTime
	spec.UpdateTime = time.Now()
	spec.Status = existing.Status
	spec.LastRun = existing.LastRun
	spec.Logs = existing.Logs

	w.workflows[spec.Name] = spec
	
	// 添加更新日志
	w.addWorkflowLog(spec, "info", "工作流配置已更新")
	return nil
}

func (w *workflowServiceImpl) DeleteWorkflow(ctx context.Context, name string) error {
	w.Lock()
	defer w.Unlock()

	if _, exists := w.workflows[name]; !exists {
		return fmt.Errorf("工作流 %s 不存在", name)
	}

	delete(w.workflows, name)
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
} 