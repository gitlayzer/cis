package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"cis/pkg/service"
	"cis/pkg/types"
	"cis/pkg/middleware"
)

type UserHandler struct {
	svc service.UserService
}

func NewUserHandler(svc service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.StandardResponse{
			Code:    400,
			Message: "无效的请求参数",
		})
		return
	}

	if err := h.svc.Register(c.Request.Context(), req.Username, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, types.StandardResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.StandardResponse{
		Code:    200,
		Message: "注册成功",
	})
}

func (h *UserHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.StandardResponse{
			Code:    400,
			Message: "无效的请求参数",
		})
		return
	}

	user, err := h.svc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, types.StandardResponse{
			Code:    401,
			Message: err.Error(),
		})
		return
	}

	token, err := middleware.GenerateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.StandardResponse{
			Code:    500,
			Message: "生成令牌失败",
		})
		return
	}

	c.JSON(http.StatusOK, types.StandardResponse{
		Code:    200,
		Message: "登录成功",
		Data: map[string]string{
			"token": token,
		},
	})
} 