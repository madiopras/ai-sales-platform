package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Data      any    `json:"data,omitempty"`
	Error     *Error `json:"error,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Data: data, RequestID: RequestID(c)})
}
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Data: data, RequestID: RequestID(c)})
}
func Fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, Envelope{Error: &Error{Code: code, Message: message}, RequestID: RequestID(c)})
}
func RequestID(c *gin.Context) string { return c.GetString("request_id") }
