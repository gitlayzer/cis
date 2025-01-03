package main

import (
	"github.com/gin-gonic/gin"
	"cis/pkg/handler"
	"cis/pkg/service"
)

func main() {
	r := gin.Default()
	
	// 初始化服务和处理器
	workflowSvc := service.NewWorkflowService()
	workflowHandler := handler.NewWorkflowHandler(workflowSvc)
	
	// 注册路由
	v1 := r.Group("/api/v1")
	{
		workflows := v1.Group("/workflows")
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
	
	r.Run(":8080")
} 