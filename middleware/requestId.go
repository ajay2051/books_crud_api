// Package middleware contains Gin middleware: request IDs, logging and panic recovery.
package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"

	"golang_crud/utils"
)

const requestIDHeader = "X-Request-ID"

// RequestID gives every request a unique ID. It reuses a valid incoming X-Request-ID
// (so IDs can be traced across services), echoes it in the response header, and makes
// it available to handlers, logs and error responses.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if !validRequestID(id) {
			id = newRequestID()
		}
		c.Set(utils.RequestIDKey, id)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// validRequestID accepts 1-64 chars of [A-Za-z0-9_-], so clients can't inject odd data into logs.
func validRequestID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
