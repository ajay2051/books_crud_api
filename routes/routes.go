// Package router maps URLs to handlers.
package router

import (
	"github.com/gin-gonic/gin"

	"golang_crud/handlers"
)

func New(books *handlers.BookHandler, health *handlers.HealthHandler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	_ = r.SetTrustedProxies(nil) // don't trust X-Forwarded-For unless you configure a proxy

	r.GET("/health", health.Check)

	v1 := r.Group("/api/v1") // versioned, so you can add /api/v2 later without breaking clients
	{
		b := v1.Group("/books")
		b.POST("", books.Create)
		b.GET("", books.List)
		b.GET("/:id", books.Get)
		b.PUT("/:id", books.Update)
		b.DELETE("/:id", books.Delete)
	}
	return r
}
