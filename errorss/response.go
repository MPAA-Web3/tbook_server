package errorss

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// Json 用于标准成功响应格式
type Json struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// CustomError 用于标准错误响应格式
type CustomError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Err  string `json:"error"`
}

// 错误代码和消息映射
var errorCodeTextMap = map[int]string{
	400: "Bad Request",
	401: "Unauthorized",
	403: "Forbidden",
	404: "Not Found",
	500: "Internal Server Error",
}

// JsonSuccess 统一成功返回程序
func JsonSuccess(ctx *gin.Context, data interface{}) {
	ctx.Status(http.StatusOK)
	ctx.JSON(http.StatusOK, Json{
		Code: 200,
		Msg:  "success",
		Data: data,
	})
	ctx.Abort()
}

// HandleError 统一错误处理程序
func HandleError(c *gin.Context, code int, err error) {
	c.Status(http.StatusOK)
	c.JSON(http.StatusOK, CustomError{
		Code: code,
		Msg:  errorCodeTextMap[code],
		Err:  err.Error(),
	})
	c.Abort()
}
