package handler

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"cis/pkg/types"
	"cis/pkg/service"
)

type WorkflowHandler struct {
	svc service.WorkflowService
}

func NewWorkflowHandler(svc service.WorkflowService) *WorkflowHandler {
	return &WorkflowHandler{svc: svc}
}

func (h *WorkflowHandler) CreateWorkflow(c *gin.Context) {
	var spec types.WorkflowSpec
	if err := c.ShouldBindJSON(&spec); err != nil {
		c.JSON(http.StatusBadRequest, types.StandardResponse{
			Code:    400,
			Message: "无效的请求参数",
			Data:    nil,
		})
		return
	}

	if err := h.svc.CreateWorkflow(c.Request.Context(), &spec); err != nil {
		c.JSON(http.StatusInternalServerError, types.StandardResponse{
			Code:    500,
			Message: "创建工作流失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, types.StandardResponse{
		Code:    200,
		Message: "创建工作流成功",
		Data:    spec,
	})
}

func (h *WorkflowHandler) ListWorkflows(c *gin.Context) {
	workflows, err := h.svc.ListWorkflows(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.StandardResponse{
			Code:    500,
			Message: "获取工作流列表失败",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, types.StandardResponse{
		Code:    200,
		Message: "获取工作流列表成功",
		Data:    workflows,
	})
}

func (h *WorkflowHandler) GetWorkflow(c *gin.Context) {
	name := c.Param("name")
	workflow, err := h.svc.GetWorkflow(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, types.StandardResponse{
			Code:    404,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, types.StandardResponse{
		Code:    200,
		Message: "获取工作流成功",
		Data:    workflow,
	})
}

func (h *WorkflowHandler) UpdateWorkflow(c *gin.Context) {
	name := c.Param("name")
	var spec types.WorkflowSpec
	if err := c.ShouldBindJSON(&spec); err != nil {
		c.JSON(http.StatusBadRequest, types.StandardResponse{
			Code:    400,
			Message: "无效的请求参数",
			Data:    nil,
		})
		return
	}

	if spec.Name != name {
		c.JSON(http.StatusBadRequest, types.StandardResponse{
			Code:    400,
			Message: "工作流名称不匹配",
			Data:    nil,
		})
		return
	}

	if err := h.svc.UpdateWorkflow(c.Request.Context(), &spec); err != nil {
		c.JSON(http.StatusInternalServerError, types.StandardResponse{
			Code:    500,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, types.StandardResponse{
		Code:    200,
		Message: "更新工作流成功",
		Data:    spec,
	})
}

func (h *WorkflowHandler) DeleteWorkflow(c *gin.Context) {
	name := c.Param("name")
	if err := h.svc.DeleteWorkflow(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, types.StandardResponse{
			Code:    500,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, types.StandardResponse{
		Code:    200,
		Message: "删除工作流成功",
		Data:    nil,
	})
}

func (h *WorkflowHandler) ExecuteWorkflow(c *gin.Context) {
	name := c.Param("name")
	if err := h.svc.ExecuteWorkflow(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, types.StandardResponse{
			Code:    500,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, types.StandardResponse{
		Code:    200,
		Message: "工作流开始执行",
		Data:    nil,
	})
}

func (h *WorkflowHandler) CancelWorkflow(c *gin.Context) {
	name := c.Param("name")
	if err := h.svc.CancelWorkflow(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, types.StandardResponse{
			Code:    500,
			Message: err.Error(),
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, types.StandardResponse{
		Code:    200,
		Message: "工作流已取消",
		Data:    nil,
	})
} 