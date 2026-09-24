package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const unmatchedPath = "unmatched"

type Recorder interface {
	ObserveRequest(method, path, status string, d time.Duration)
}

func Metrics(recorder Recorder) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		path := c.FullPath()
		if path == "" {
			path = unmatchedPath
		}

		recorder.ObserveRequest(
			c.Request.Method,
			path,
			strconv.Itoa(c.Writer.Status()),
			time.Since(start),
		)
	}
}
