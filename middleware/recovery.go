package middleware

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"golang_crud/utils"
)

// Recovery catches panics in handlers, logs the stack trace, and returns a clean JSON 500
// instead of crashing the request (or leaking internals to the client).
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered",
					"request_id", utils.RequestID(c),
					"panic", fmt.Sprint(rec),
					"stack", string(debug.Stack()),
				)
				utils.Error(c, utils.NewInternal(fmt.Errorf("panic: %v", rec)))
			}
		}()
		c.Next()
	}
}
