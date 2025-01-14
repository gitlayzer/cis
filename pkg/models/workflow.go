package models

import (
	"encoding/json"
	"gorm.io/gorm"
	"cis/pkg/types"
	"fmt"
)

// Workflow 数据库模型
type Workflow struct {
	gorm.Model
	Name       string `gorm:"type:varchar(255);uniqueIndex:idx_name_user"`
	UserID     uint   `gorm:"uniqueIndex:idx_name_user"`
	User       User   `gorm:"foreignKey:UserID"`
	Source     string `gorm:"type:text"` // JSON 存储
	Targets    string `gorm:"type:text"` // JSON 存储
	Images     string `gorm:"type:text"` // JSON 存储
	Status     string `gorm:"type:varchar(32)"`
	LastRun    string `gorm:"type:text"` // JSON 存储
	Logs       string `gorm:"type:text"` // JSON 存储
	Schedule   string `gorm:"type:text"` // JSON 存储
}

// ToSpec 转换为 WorkflowSpec
func (w *Workflow) ToSpec() (*types.WorkflowSpec, error) {
	spec := &types.WorkflowSpec{
		Name:       w.Name,
		UserID:     w.UserID,
		Status:     types.WorkflowStatus(w.Status),
		CreateTime: w.CreatedAt,
		UpdateTime: w.UpdatedAt,
		Logs:       make([]types.WorkflowLog, 0), // 初始化空日志数组
	}

	// 解析 Source
	if err := json.Unmarshal([]byte(w.Source), &spec.Source); err != nil {
		return nil, err
	}

	// 解析 Targets
	if err := json.Unmarshal([]byte(w.Targets), &spec.Targets); err != nil {
		return nil, err
	}

	// 解析 Images
	if err := json.Unmarshal([]byte(w.Images), &spec.Images); err != nil {
		return nil, err
	}

	// 解析 LastRun
	if w.LastRun != "" {
		var lastRun types.WorkflowRun
		if err := json.Unmarshal([]byte(w.LastRun), &lastRun); err != nil {
			return nil, err
		}
		spec.LastRun = &lastRun
	}

	// 解析 Logs
	if w.Logs != "" {
		var logs []types.WorkflowLog
		if err := json.Unmarshal([]byte(w.Logs), &logs); err != nil {
			fmt.Printf("解析日志失败: %v\n", err)
		} else {
			spec.Logs = logs
		}
	}

	// 解析 Schedule
	if w.Schedule != "" {
		var schedule types.Schedule
		if err := json.Unmarshal([]byte(w.Schedule), &schedule); err != nil {
			return nil, err
		}
		spec.Schedule = &schedule
	}

	return spec, nil
}

// FromSpec 从 WorkflowSpec 转换
func (w *Workflow) FromSpec(spec *types.WorkflowSpec) error {
	w.Name = spec.Name
	w.Status = string(spec.Status)
	w.UserID = spec.UserID

	// 序列化 Source
	source, err := json.Marshal(spec.Source)
	if err != nil {
		return err
	}
	w.Source = string(source)

	// 序列化 Targets
	targets, err := json.Marshal(spec.Targets)
	if err != nil {
		return err
	}
	w.Targets = string(targets)

	// 序列化 Images
	images, err := json.Marshal(spec.Images)
	if err != nil {
		return err
	}
	w.Images = string(images)

	// 序列化 LastRun
	if spec.LastRun != nil {
		lastRun, err := json.Marshal(spec.LastRun)
		if err != nil {
			return err
		}
		w.LastRun = string(lastRun)
	}

	// 序列化 Logs
	logs, err := json.Marshal(spec.Logs)
	if err != nil {
		return err
	}
	w.Logs = string(logs)

	// 序列化 Schedule
	if spec.Schedule != nil {
		schedule, err := json.Marshal(spec.Schedule)
		if err != nil {
			return err
		}
		w.Schedule = string(schedule)
	}

	return nil
} 