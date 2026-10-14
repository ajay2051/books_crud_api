// Package models holds domain types and domain errors.
// No database or HTTP codes lives here

package models

import (
	"errors"
	"time"
)

// ErrNotFound is returned by the data layer when a record doesn't exist.
// The HTTP layer maps it to a 404.

var ErrNotFound = errors.New("Book Not Found")

type Book struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Author    string    `json:"author"`
	Year      int       `json:"year"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
