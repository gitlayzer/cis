package db

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"cis/pkg/models"
)

var DB *gorm.DB

func InitDB() error {
	dsn := fmt.Sprintf("root:123456@tcp(10.0.0.12:3306)/cis?charset=utf8mb4&parseTime=True&loc=Local")
	
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %v", err)
	}

	// 自动迁移数据库结构
	if err := DB.AutoMigrate(&models.Workflow{}); err != nil {
		return fmt.Errorf("数据库迁移失败: %v", err)
	}

	return nil
} 