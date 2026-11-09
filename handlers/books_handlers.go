// Package handlers deals with HTTP only: bind + validate the request, call the store,
// write the response. No SQL here.
package handlers

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"golang_crud/models"
	"golang_crud/utils"
)

// Success messages live in one place so they're easy to change or translate.
const (
	msgBookCreated  = "Book created successfully"
	msgBooksFetched = "Books retrieved successfully"
	msgBookFetched  = "Book retrieved successfully"
	msgBookUpdated  = "Book updated successfully"
	msgBookDeleted  = "Book deleted successfully"
)

// BookStore is what the handlers need from the data layer.
// An interface lets tests plug in a fake instead of Postgres.
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

// ---- request types ----------------------------------------------------

// bookRequest is the JSON we accept from clients. Keeping it separate from models.Book
// means clients can't set fields like id or created_at.
//
// Validation rules live in the `binding` tags (go-playground/validator syntax).
type bookRequest struct {
	Title  string `json:"title" binding:"required,notblank,max=255"`
	Author string `json:"author" binding:"required,notblank,max=255"`
	Year   int    `json:"year" binding:"omitempty,gte=1,notfuture"`
}

// Normalize runs before validation: trim stray whitespace.
func (r *bookRequest) Normalize() {
	r.Title = strings.TrimSpace(r.Title)
	r.Author = strings.TrimSpace(r.Author)
}

func (r *bookRequest) toModel() models.Book {
	return models.Book{Title: r.Title, Author: r.Author, Year: r.Year}
}

// listQuery holds ?limit= and ?offset=; defaults apply when they are absent.
type listQuery struct {
	Limit  int `form:"limit,default=20" binding:"gte=1,lte=100"`
	Offset int `form:"offset,default=0" binding:"gte=0,lte=1000000"`
}

// ---- handlers ---------------------------------------------------------

// POST /api/v1/books
func (h *BookHandler) Create(c *gin.Context) {
	var req bookRequest
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Error(c, err)
		return
	}

	book := req.toModel()
	if err := h.store.Create(c.Request.Context(), &book); err != nil {
		utils.Error(c, translateError(err))
		return
	}
	utils.Created(c, msgBookCreated, book)
}

// GET /api/v1/books?limit=20&offset=0
func (h *BookHandler) List(c *gin.Context) {
	var q listQuery
	if err := utils.BindQuery(c, &q); err != nil {
		utils.Error(c, err)
		return
	}

	books, err := h.store.GetAll(c.Request.Context(), q.Limit, q.Offset)
	if err != nil {
		utils.Error(c, translateError(err))
		return
	}
	utils.SuccessWithMeta(c, 200, msgBooksFetched, books,
		gin.H{"limit": q.Limit, "offset": q.Offset, "count": len(books)})
}

// GET /api/v1/books/:id
func (h *BookHandler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	book, err := h.store.GetByID(c.Request.Context(), id)
	if err != nil {
		utils.Error(c, translateError(err))
		return
	}
	utils.OK(c, msgBookFetched, book)
}

// PUT /api/v1/books/:id
func (h *BookHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req bookRequest
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Error(c, err)
		return
	}

	in := req.toModel()
	book, err := h.store.Update(c.Request.Context(), id, &in)
	if err != nil {
		utils.Error(c, translateError(err))
		return
	}
	utils.OK(c, msgBookUpdated, book)
}

// DELETE /api/v1/books/:id
func (h *BookHandler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := h.store.Delete(c.Request.Context(), id); err != nil {
		utils.Error(c, translateError(err))
		return
	}
	utils.OK(c, msgBookDeleted, nil)
}

// ---- helpers ----------------------------------------------------------

// parseID reads :id from the URL. On failure it writes a 422 and returns false.
func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		utils.Error(c, utils.NewValidationError([]utils.ErrorDetail{
			{Field: "id", Message: "id must be a positive integer"},
		}))
		return 0, false
	}
	return id, true
}

// translateError maps errors from the data layer to client-facing AppErrors.
// Anything unrecognised is returned unchanged; utils.Error turns it into a generic 500
// and logs the real cause.
func translateError(err error) error {
	switch {
	case errors.Is(err, models.ErrNotFound):
		return utils.NewNotFound("Book not found")
	default:
		return err
	}
}
