// Package routes maps URLs to handlers and installs middleware.
package routes

import (
	"github.com/gin-gonic/gin"

	"golang_crud/handlers"
	"golang_crud/middleware"
	"golang_crud/utils"
)

func New(books *handlers.BookHandler, health *handlers.HealthHandler) *gin.Engine {
	r := gin.New()
	_ = r.SetTrustedProxies(nil) // don't trust X-Forwarded-For unless you configure a proxy
	r.HandleMethodNotAllowed = true

	// Order matters: RequestID first (so everything can use it), Logger next
	// (so it also logs recovered panics), Recovery last.
	r.Use(middleware.RequestID(), middleware.Logger(), middleware.Recovery())

	// Unknown URLs / wrong methods get the same JSON envelope as every other error.
	r.NoRoute(func(c *gin.Context) {
		utils.Error(c, utils.NewNotFound("The requested route does not exist"))
	})
	r.NoMethod(func(c *gin.Context) {
		utils.Error(c, utils.NewMethodNotAllowed())
	})

	r.GET("/health", health.Check)

	v1 := r.Group("/api/v1")
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
