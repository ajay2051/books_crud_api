package test

import (
	"net/http"
	"testing"
	"time"

	"golang_crud/utils"
)

// Books CRUD behaviour: success responses, validation and not-found errors.
// Every request here carries a valid access token (see sendAuthed).

func TestCreateBook_Success(t *testing.T) {
	w := sendAuthed(t, newTestRouter(), http.MethodPost, "/api/v1/books",
		`{"title":"  Go in Action  ","author":"Kennedy","year":2015}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	resp := decode(t, w)
	if !resp.Success || resp.Message != "Book created successfully" {
		t.Fatalf("unexpected envelope: %s", w.Body.String())
	}
	data, _ := resp.Data.(map[string]any)
	if data["title"] != "Go in Action" { // whitespace trimmed by Normalize
		t.Fatalf("title not trimmed: %v", data["title"])
	}
}

func TestDeleteBook(t *testing.T) {
	r := newTestRouter()
	sendAuthed(t, r, http.MethodPost, "/api/v1/books", `{"title":"A","author":"B"}`)

	w := sendAuthed(t, r, http.MethodDelete, "/api/v1/books/1", "")
	if w.Code != http.StatusOK || decode(t, w).Message != "Book deleted successfully" {
		t.Fatalf("unexpected delete response: %d %s", w.Code, w.Body.String())
	}
	expectError(t, sendAuthed(t, r, http.MethodDelete, "/api/v1/books/1", ""), http.StatusNotFound, utils.CodeNotFound)
}

func TestListBooks_HasMeta(t *testing.T) {
	r := newTestRouter()
	sendAuthed(t, r, http.MethodPost, "/api/v1/books", `{"title":"A","author":"B"}`)

	w := sendAuthed(t, r, http.MethodGet, "/api/v1/books?limit=5", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	meta, _ := decode(t, w).Meta.(map[string]any)
	if meta["limit"] != float64(5) || meta["count"] != float64(1) {
		t.Fatalf("unexpected meta: %v", meta)
	}
}

func TestCreateBook_FieldValidation(t *testing.T) {
	w := sendAuthed(t, newTestRouter(), http.MethodPost, "/api/v1/books",
		`{"title":"   ","author":"","year":-5}`)

	resp := expectError(t, w, http.StatusUnprocessableEntity, utils.CodeValidation)
	fields := detailFields(resp)
	for _, f := range []string{"title", "author", "year"} {
		if fields[f] == "" {
			t.Errorf("expected a validation message for %q, got %v", f, fields)
		}
	}
}

func TestCreateBook_FutureYear(t *testing.T) {
	resp := expectError(t,
		sendAuthed(t, newTestRouter(), http.MethodPost, "/api/v1/books", `{"title":"A","author":"B","year":9999}`),
		http.StatusUnprocessableEntity, utils.CodeValidation)
	if detailFields(resp)["year"] != "year cannot be in the future" {
		t.Fatalf("unexpected details: %v", resp.Error.Details)
	}
}

func TestCreateBook_UnknownField(t *testing.T) {
	resp := expectError(t,
		sendAuthed(t, newTestRouter(), http.MethodPost, "/api/v1/books", `{"title":"A","author":"B","isbn":"x"}`),
		http.StatusUnprocessableEntity, utils.CodeValidation)
	if detailFields(resp)["isbn"] != "unknown field" {
		t.Fatalf("unexpected details: %v", resp.Error.Details)
	}
}

func TestCreateBook_WrongFieldType(t *testing.T) {
	resp := expectError(t,
		sendAuthed(t, newTestRouter(), http.MethodPost, "/api/v1/books", `{"title":"A","author":"B","year":"abc"}`),
		http.StatusUnprocessableEntity, utils.CodeValidation)
	if detailFields(resp)["year"] != "year must be a number" {
		t.Fatalf("unexpected details: %v", resp.Error.Details)
	}
}

func TestCreateBook_MalformedJSON(t *testing.T) {
	expectError(t, sendAuthed(t, newTestRouter(), http.MethodPost, "/api/v1/books", `{"title":`),
		http.StatusBadRequest, utils.CodeInvalidJSON)
}

func TestCreateBook_EmptyBody(t *testing.T) {
	expectError(t, sendAuthed(t, newTestRouter(), http.MethodPost, "/api/v1/books", ``),
		http.StatusBadRequest, utils.CodeInvalidJSON)
}

func TestCreateBook_WrongContentType(t *testing.T) {
	token, _, _ := newTokens(time.Hour).Issue(1, "tester@example.com")
	w := send(newTestRouter(), http.MethodPost, "/api/v1/books", `{"title":"A","author":"B"}`, "text/plain", "Bearer "+token)
	expectError(t, w, http.StatusUnsupportedMediaType, utils.CodeUnsupportedMediaType)
}

func TestGetBook_InvalidID(t *testing.T) {
	resp := expectError(t, sendAuthed(t, newTestRouter(), http.MethodGet, "/api/v1/books/abc", ""),
		http.StatusUnprocessableEntity, utils.CodeValidation)
	if detailFields(resp)["id"] == "" {
		t.Fatalf("expected a message for id, got %v", resp.Error.Details)
	}
}

func TestListBooks_InvalidQuery(t *testing.T) {
	r := newTestRouter()

	resp := expectError(t, sendAuthed(t, r, http.MethodGet, "/api/v1/books?limit=1000", ""),
		http.StatusUnprocessableEntity, utils.CodeValidation)
	if detailFields(resp)["limit"] == "" {
		t.Fatalf("expected a message for limit, got %v", resp.Error.Details)
	}

	expectError(t, sendAuthed(t, r, http.MethodGet, "/api/v1/books?limit=abc", ""),
		http.StatusBadRequest, utils.CodeBadRequest)
}

func TestGetBook_NotFound(t *testing.T) {
	resp := expectError(t, sendAuthed(t, newTestRouter(), http.MethodGet, "/api/v1/books/999", ""),
		http.StatusNotFound, utils.CodeNotFound)
	if resp.Error.Message != "Book not found" {
		t.Fatalf("unexpected message: %s", resp.Error.Message)
	}
}
