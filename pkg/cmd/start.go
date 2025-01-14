package cmd

import (
	"cis/pkg/db"
	"cis/pkg/handler"
	"cis/pkg/middleware"
	"cis/pkg/service"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

var (
	dbHost string
	dbPort string
	dbUser string
	dbPass string
	dbName string
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动服务",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 初始化数据库配置
		config := db.DBConfig{
			Host:     db.GetConfigValue(dbHost, "DB_HOST", "localhost"),
			Port:     db.GetConfigValue(dbPort, "DB_PORT", "3306"),
			User:     db.GetConfigValue(dbUser, "DB_USER", "root"),
			Password: db.GetConfigValue(dbPass, "DB_PASSWORD", "123456"),
			DBName:   db.GetConfigValue(dbName, "DB_NAME", "cis"),
		}

		// 初始化数据库
		if err := db.InitDB(config); err != nil {
			return err
		}

		r := gin.Default()

		// 初始化服务和处理器
		workflowSvc := service.NewWorkflowService()
		workflowHandler := handler.NewWorkflowHandler(workflowSvc)
		userSvc := service.NewUserService()
		userHandler := handler.NewUserHandler(userSvc)

		// 注册路由
		v1 := r.Group("/api/v1")
		{
			// 用户相关路由（无需认证）
			users := v1.Group("/users")
			{
				users.POST("/register", userHandler.Register)
				users.POST("/login", userHandler.Login)
			}

			// 需要认证的路由
			auth := v1.Group("/")
			auth.Use(middleware.AuthMiddleware())
			{
				workflows := auth.Group("/workflows")
				{
					workflows.POST("/", workflowHandler.CreateWorkflow)
					workflows.GET("/", workflowHandler.ListWorkflows)
					workflows.GET("/:name", workflowHandler.GetWorkflow)
					workflows.PUT("/:name", workflowHandler.UpdateWorkflow)
					workflows.DELETE("/:name", workflowHandler.DeleteWorkflow)
					workflows.POST("/:name/execute", workflowHandler.ExecuteWorkflow)
					workflows.POST("/:name/cancel", workflowHandler.CancelWorkflow)
				}
			}
		}

		return r.Run(":8080")
	},
}

func init() {
	// 添加数据库相关的命令行参数
	startCmd.Flags().StringVar(&dbHost, "db-host", "", "数据库主机地址")
	startCmd.Flags().StringVar(&dbPort, "db-port", "", "数据库端口")
	startCmd.Flags().StringVar(&dbUser, "db-user", "", "数据库用户名")
	startCmd.Flags().StringVar(&dbPass, "db-pass", "", "数据库密码")
	startCmd.Flags().StringVar(&dbName, "db-name", "", "数据库名称")

	rootCmd.AddCommand(startCmd)
}
