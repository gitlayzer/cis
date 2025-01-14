package types

import "time"

// WorkflowStatus 定义工作流状态
type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"   // 待执行
	WorkflowStatusRunning   WorkflowStatus = "running"   // 执行中
	WorkflowStatusSuccess   WorkflowStatus = "success"   // 执行成功
	WorkflowStatusFailed    WorkflowStatus = "failed"    // 执行失败
)

// WorkflowSpec 定义同步工作流配置
type WorkflowSpec struct {
	Name       string        `json:"name"`
	UserID     uint         `json:"user_id,omitempty"`
	Source     RegistryAuth  `json:"source"`
	Targets    []RegistryAuth `json:"targets"`
	Images     []string      `json:"images"`
	Status     WorkflowStatus `json:"status"`     // 工作流状态
	CreateTime time.Time     `json:"createTime"`  // 创建时间
	UpdateTime time.Time     `json:"updateTime"`  // 更新时间
	LastRun    *WorkflowRun  `json:"lastRun"`    // 最后一次运行记录
	Logs       []WorkflowLog `json:"logs"`       // 工作流日志
	Schedule   *Schedule     `json:"schedule,omitempty"`  // 定时配置，可选
}

// WorkflowRun 定义工作流运行记录
type WorkflowRun struct {
	StartTime  time.Time     `json:"startTime"`  // 开始时间
	EndTime    time.Time     `json:"endTime"`    // 结束时间
	Status     WorkflowStatus `json:"status"`    // 运行状态
	Message    string        `json:"message"`    // 运行结果信息
}

// WorkflowLog 定义工作流日志
type WorkflowLog struct {
	Timestamp time.Time `json:"timestamp"` // 日志时间
	Level     string    `json:"level"`     // 日志级别 (info/warn/error)
	Message   string    `json:"message"`   // 日志内容
}

// RegistryAuth 定义镜像仓库认证信息
type RegistryAuth struct {
	URL       string `json:"url"`
	Auth      *Auth  `json:"auth,omitempty"`
	Insecure  bool   `json:"insecure"`
	Namespace string `json:"namespace,omitempty"`
}

type Auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// StandardResponse 定义标准响应格式
type StandardResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// Schedule 定义定时执行配置
type Schedule struct {
	Cron     string    `json:"cron"`      // Cron 表达式
	Enabled  bool      `json:"enabled"`    // 是否启用定时
	NextRun  time.Time `json:"nextRun"`    // 下次执行时间
} 