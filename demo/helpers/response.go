package helpers

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type R struct {
	ErrorCode int         `json:"error_code"`
	ErrorNote string      `json:"error_note"`
	Data      interface{} `json:"data"`
}

func RespondOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, R{
		Data: data,
	})
}

func RespondError(c *gin.Context, code int, note string) {
	c.JSON(code, R{
		ErrorCode: code,
		ErrorNote: note,
	})
}
