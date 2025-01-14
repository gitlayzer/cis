package db

import (
	"fmt"
	"os"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"cis/pkg/models"
)

var DB *gorm.DB

// DBConfig 数据库配置
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// InitDB 初始化数据库连接
func InitDB(config DBConfig) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.DBName,
	)
	
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %v", err)
	}

	// 自动迁移数据库结构
	if err := DB.AutoMigrate(&models.User{}, &models.Workflow{}); err != nil {
		return fmt.Errorf("数据库迁移失败: %v", err)
	}

	return nil
}

// GetConfigValue 获取配置值，按优先级返回：命令行参数 > 环境变量 > 默认值
func GetConfigValue(flagValue string, envKey string, defaultValue string) string {
	if flagValue != "" {
		return flagValue
	}
	
	if envValue := os.Getenv(envKey); envValue != "" {
		return envValue
	}
	
	return defaultValue
} 