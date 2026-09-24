package middleware

import (
	"crypto/rand"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"

	"graph-code-challenge/internal/requestid"
)

const maxRequestIDLength = 128

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader(requestid.Header))
		if !validRequestID(id) {
			id = rand.Text()
		}

		c.Header(requestid.Header, id)
		c.Request = c.Request.WithContext(requestid.NewContext(c.Request.Context(), id))
		c.Next()
	}
}

func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLength {
		return false
	}

	for _, character := range id {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '-' && character != '_' && character != '.' {
			return false
		}
	}

	return true
}
