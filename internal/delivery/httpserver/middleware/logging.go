package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"

	"graph-code-challenge/internal/logger"
)

func RequestLogger(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		if c.Request.URL.Path == "/metrics" {
			return
		}

		path := c.FullPath()
		if path == "" {
			path = unmatchedPath
		}

		log.Info(
			c.Request.Context(),
			"http request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration", time.Since(start),
			"client_ip", c.ClientIP(),
		)
	}
}

func Recovery(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error(
					c.Request.Context(),
					"panic recovered",
					"panic", fmt.Sprint(recovered),
					"stack", string(debug.Stack()),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "internal server error"})
			}
		}()

		c.Next()
	}
}
