package service

import (
	"context"
	"cis/pkg/types"
)

type WorkflowService interface {
	// CreateWorkflow 创建新的工作流
	CreateWorkflow(ctx context.Context, spec *types.WorkflowSpec) error
	
	// ListWorkflows 列出所有工作流
	ListWorkflows(ctx context.Context) ([]*types.WorkflowSpec, error)
	
	// GetWorkflow 获取特定工作流
	GetWorkflow(ctx context.Context, name string) (*types.WorkflowSpec, error)
	
	// UpdateWorkflow 更新工作流
	UpdateWorkflow(ctx context.Context, spec *types.WorkflowSpec) error
	
	// DeleteWorkflow 删除工作流
	DeleteWorkflow(ctx context.Context, name string) error
	
	// ExecuteWorkflow 执行工作流
	ExecuteWorkflow(ctx context.Context, name string) error
	
	// CancelWorkflow 取消工作流执行
	CancelWorkflow(ctx context.Context, name string) error
} 