// Package repository contains all SQL queries. Nothing else writes SQL.

package repository

import (
	"context"
	"database/sql"
	"errors"

	"golang_crud/models"
)

const bookColumns = `id, title, author, year, created_at, updated_at`

type BookRepository struct {
	db *sql.DB
}

func NewBookRepository(db *sql.DB) *BookRepository {
	return &BookRepository{db: db}
}

// scanner is satisfied by both *sql.Row and *sql.Rows, so one helper serves all queries.

type scanner interface {
	Scan(dest ...any) error
}

func scanBook(s scanner) (models.Book, error) {
	var b models.Book
	err := s.Scan(&b.ID, &b.Title, &b.Author, &b.Year, &b.CreatedAt, &b.UpdatedAt)
	return b, err
}

// $1, $2... are placeholders: ALWAYS use them (never string-concat SQL) to avoid SQL injection.

func (r *BookRepository) Create(ctx context.Context, b *models.Book) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO books (title, author, year) VALUES ($1, $2, $3)
				 RETURNING id, created_at, updated_at`,
		b.Title, b.Author, b.Year,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)

}

func (r *BookRepository) GetAll(ctx context.Context, limit, offset int) ([]models.Book, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+bookColumns+` FROM books ORDER BY id LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close() // always close rows, or you leak connections

	books := []models.Book{} // empty slice (not nil) so JSON returns [] instead of null
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}

func (r *BookRepository) GetByID(ctx context.Context, id int64) (*models.Book, error) {
	b, err := scanBook(r.db.QueryRowContext(ctx,
		`SELECT `+bookColumns+` FROM books WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *BookRepository) Update(ctx context.Context, id int64, in *models.Book) (*models.Book, error) {
	b, err := scanBook(r.db.QueryRowContext(ctx,
		`UPDATE books
		 SET title = $1, author = $2, year = $3, updated_at = NOW()
		 WHERE id = $4
		 RETURNING `+bookColumns,
		in.Title, in.Author, in.Year, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *BookRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM books WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return models.ErrNotFound
	}
	return nil
}
