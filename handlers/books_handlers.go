// Package handlers deals with HTTP only; read the request, call the store.
// Write the response. No SQL here.

package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang_crud/models"
)

// BookStore is what the handlers need from the data layer.
// Defining  it here (an interface) lets tests plug in a fake instead of Postgres.

type BookStore interface {
	Create(ctx context.Context, b *models.Book) error
	GetAll(ctx context.Context, limit, offset int) ([]models.Book, error)
	GetByID(ctx context.Context, id int64) (*models.Book, error)
	Update(ctx context.Context, id int64, b *models.Book) (*models.Book, error)
	Delete(ctx context.Context, id int64) error
}

type BookHandler struct {
	store BookStore
}

func NewBookHandler(store BookStore) *BookHandler {
	return &BookHandler{store: store}
}

// bookRequest is the JSON we accept from clients. Keeping it separate from
// models.Book means clients can't set fields like id or created_at.
type bookRequest struct {
	Title  string `json:"title" binding:"required"`
	Author string `json:"author" binding:"required"`
	Year   int    `json:"year" binding:"gte=0,lte=9999"`
}

func (r bookRequest) toModel() models.Book {
	return models.Book{Title: r.Title, Author: r.Author, Year: r.Year}
}

// POST /api/v1/books
func (h *BookHandler) Create(c *gin.Context) {
	var req bookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	book := req.toModel()
	if err := h.store.Create(c.Request.Context(), &book); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, book)
}

// GET /api/v1/books?limit=20&offset=0
func (h *BookHandler) List(c *gin.Context) {
	limit := queryInt(c, "limit", 20, 1, 100)
	offset := queryInt(c, "offset", 0, 0, 1_000_000)

	books, err := h.store.GetAll(c.Request.Context(), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": books, "limit": limit, "offset": offset})
}

// GET /api/v1/books/:id
func (h *BookHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	book, err := h.store.GetByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, book)
}

// PUT /api/v1/books/:id
func (h *BookHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req bookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	in := req.toModel()
	book, err := h.store.Update(c.Request.Context(), id, &in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, book)
}

// DELETE /api/v1/books/:id
func (h *BookHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.store.Delete(c.Request.Context(), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent) // 204, no body
}

// --- helpers ---------------------------------------------------------

// parseID reads :id from the URL. On failure it writes a 400 and returns false.
func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return id, true
}

// queryInt reads an integer query parameter, falling back to def and clamping to [min, max].
func queryInt(c *gin.Context, key string, def, min, max int) int {
	v, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// respondError maps errors to HTTP status codes in one place.
func respondError(c *gin.Context, err error) {
	if errors.Is(err, models.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	// Log the real error for ourselves, but don't leak details to the client.
	log.Printf("internal error: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
