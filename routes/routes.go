// Package routes maps URLs to handlers and installs middleware.
package routes

import (
	"github.com/gin-gonic/gin"

	"golang_crud/handlers"
	"golang_crud/middleware"
	"golang_crud/utils"
)

// Handlers groups everything the router needs, so New's signature stays stable as the app grows.
type Handlers struct {
	Books  *handlers.BookHandler
	Health *handlers.HealthHandler
	Auth   *handlers.AuthHandler
}

// New builds the router. authLimiter (optional, may be nil) is applied to register and login.
func New(h Handlers, verifier middleware.TokenVerifier, authLimiter gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	_ = r.SetTrustedProxies(nil) // don't trust X-Forwarded-For unless you configure a proxy
	r.HandleMethodNotAllowed = true

	// Order matters: RequestID first (so everything can use it), Logger next
	// (so it also logs recovered panics), Recovery last.
	r.Use(middleware.RequestID(), middleware.Logger(), middleware.Recovery())

	r.NoRoute(func(c *gin.Context) {
		utils.Error(c, utils.NewNotFound("The requested route does not exist"))
	})
	r.NoMethod(func(c *gin.Context) {
		utils.Error(c, utils.NewMethodNotAllowed())
	})

	// ---- public ----
	r.GET("/health", h.Health.Check)

	v1 := r.Group("/api/v1")

	withLimit := func(handler gin.HandlerFunc) []gin.HandlerFunc {
		if authLimiter == nil {
			return []gin.HandlerFunc{handler}
		}
		return []gin.HandlerFunc{authLimiter, handler}
	}
	v1.POST("/auth/register", withLimit(h.Auth.Register)...)
	v1.POST("/auth/login", withLimit(h.Auth.Login)...)

	// ---- protected: every route below needs a valid access token ----
	protected := v1.Group("", middleware.Auth(verifier))
	{
		protected.GET("/auth/me", h.Auth.Me)

		books := protected.Group("/books")
		books.POST("", h.Books.Create)
		books.GET("", h.Books.List)
		books.GET("/:id", h.Books.Get)
		books.PUT("/:id", h.Books.Update)
		books.DELETE("/:id", h.Books.Delete)
	}
	return r
}
