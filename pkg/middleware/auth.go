package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"net/http"
	"time"
	"context"
	"cis/pkg/types"
)

const (
	secretKey = "your-secret-key" // 在实际应用中应该使用环境变量
)

func GenerateToken(userID uint) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})
	return token.SignedString([]byte(secretKey))
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, types.StandardResponse{
				Code:    401,
				Message: "未授权",
				Data:    nil,
			})
			c.Abort()
			return
		}

		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(secretKey), nil
		})

		if err != nil {
			c.JSON(http.StatusUnauthorized, types.StandardResponse{
				Code:    401,
				Message: "无效的令牌",
				Data:    nil,
			})
			c.Abort()
			return
		}

		userID := uint(claims["user_id"].(float64))
		ctx := context.WithValue(c.Request.Context(), "user_id", userID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
} 