package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一 API 响应格式
type Response struct {
	Code    int         `json:"code"`    // 0=success, >0=error
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 返回成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "success", Data: data})
}

// Error 返回错误响应
func Error(c *gin.Context, httpStatus int, code int, msg string) {
	c.JSON(httpStatus, Response{Code: code, Message: msg})
}
