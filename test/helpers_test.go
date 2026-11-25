package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"golang_crud/auth"
	"golang_crud/handlers"
	"golang_crud/models"
	"golang_crud/routes"
	"golang_crud/utils"
)

// Shared fakes and helpers for every test in this package.
// Nothing here talks to a real database: the stores are in-memory fakes.

const (
	testSecret = "test-secret-test-secret-test-secret-1234"
	testIssuer = "golang_crud-test"
)

// TestMain runs once before all tests in this package.
func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil))) // keep test output readable
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// ---- fakes: no database needed ---------------------------------------

type fakeStore struct {
	books  map[int64]models.Book
	nextID int64
}

func newFakeStore() *fakeStore {
	return &fakeStore{books: map[int64]models.Book{}, nextID: 1}
}

func (f *fakeStore) Create(_ context.Context, b *models.Book) error {
	b.ID = f.nextID
	b.CreatedAt = time.Now()
	b.UpdatedAt = b.CreatedAt
	f.books[b.ID] = *b
	f.nextID++
	return nil
}

func (f *fakeStore) GetAll(_ context.Context, limit, offset int) ([]models.Book, error) {
	out := []models.Book{}
	for id := int64(1); id < f.nextID; id++ {
		if b, ok := f.books[id]; ok {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeStore) GetByID(_ context.Context, id int64) (*models.Book, error) {
	b, ok := f.books[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	return &b, nil
}

func (f *fakeStore) Update(_ context.Context, id int64, in *models.Book) (*models.Book, error) {
	b, ok := f.books[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	b.Title, b.Author, b.Year = in.Title, in.Author, in.Year
	b.UpdatedAt = time.Now()
	f.books[id] = b
	return &b, nil
}

func (f *fakeStore) Delete(_ context.Context, id int64) error {
	if _, ok := f.books[id]; !ok {
		return models.ErrNotFound
	}
	delete(f.books, id)
	return nil
}

type fakePinger struct{}

func (fakePinger) PingContext(context.Context) error { return nil }

// ---- helpers ---------------------------------------------------------

func newTokens(ttl time.Duration) *auth.TokenManager {
	return auth.NewTokenManager(testSecret, testIssuer, ttl)
}

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	tokens := newTokens(time.Hour)
	return routes.New(
		routes.Handlers{
			Books:  handlers.NewBookHandler(newFakeStore()),
			Health: handlers.NewHealthHandler(fakePinger{}),
			Auth:   handlers.NewAuthHandler(newFakeUserStore(), tokens),
		},
		tokens,
		nil, // no rate limiting in tests
	)
}

func send(r http.Handler, method, path, body, contentType, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// sendJSON sends a request with no credentials.
func sendJSON(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	return send(r, method, path, body, "application/json", "")
}

// sendAuthed sends a request with a valid access token for user 1.
func sendAuthed(t *testing.T, r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	token, _, err := newTokens(time.Hour).Issue(1, "tester@example.com")
	if err != nil {
		t.Fatal(err)
	}
	return send(r, method, path, body, "application/json", "Bearer "+token)
}

func decode(t *testing.T, w *httptest.ResponseRecorder) utils.Response {
	t.Helper()
	var resp utils.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v (%s)", err, w.Body.String())
	}
	return resp
}

func expectError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) utils.Response {
	t.Helper()
	if w.Code != status {
		t.Fatalf("want status %d, got %d (%s)", status, w.Code, w.Body.String())
	}
	resp := decode(t, w)
	if resp.Success || resp.Error == nil {
		t.Fatalf("expected an error envelope, got %s", w.Body.String())
	}
	if resp.Error.Code != code {
		t.Fatalf("want code %s, got %s", code, resp.Error.Code)
	}
	return resp
}

func detailFields(resp utils.Response) map[string]string {
	out := map[string]string{}
	for _, d := range resp.Error.Details {
		out[d.Field] = d.Message
	}
	return out
}

// fakeUserStore is an in-memory UserStore.
type fakeUserStore struct {
	byID    map[int64]models.User
	byEmail map[string]int64
	nextID  int64
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{byID: map[int64]models.User{}, byEmail: map[string]int64{}, nextID: 1}
}

func (f *fakeUserStore) Create(_ context.Context, u *models.User) error {
	if _, exists := f.byEmail[u.Email]; exists {
		return models.ErrEmailTaken
	}
	u.ID = f.nextID
	u.CreatedAt = time.Now()
	u.UpdatedAt = u.CreatedAt
	f.byID[u.ID] = *u
	f.byEmail[u.Email] = u.ID
	f.nextID++
	return nil
}

func (f *fakeUserStore) GetByEmail(_ context.Context, email string) (*models.User, error) {
	id, ok := f.byEmail[email]
	if !ok {
		return nil, models.ErrUserNotFound
	}
	u := f.byID[id]
	return &u, nil
}

func (f *fakeUserStore) GetByID(_ context.Context, id int64) (*models.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, models.ErrUserNotFound
	}
	return &u, nil
}
