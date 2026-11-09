package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"golang_crud/utils"
)

// Pinger is satisfied by *sql.DB.
type Pinger interface {
	PingContext(ctx context.Context) error
}

type HealthHandler struct {
	db Pinger
}

func NewHealthHandler(db Pinger) *HealthHandler {
	return &HealthHandler{db: db}
}

// GET /health — used by Docker, load balancers and Kubernetes to check the app is alive.
func (h *HealthHandler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		utils.Error(c, utils.NewServiceUnavailable("Database is unreachable", err))
		return
	}
	utils.OK(c, "Service is healthy", gin.H{"status": "ok"})
}
