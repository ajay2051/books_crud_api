package test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"golang_crud/utils"
)

// Register, login and "me" endpoints.

// ---- helpers ---------------------------------------------------------

func credentials(email, password string) string {
	b, _ := json.Marshal(map[string]string{"name": "Ada Lovelace", "email": email, "password": password})
	return string(b)
}

// loginBody has only email and password: the API rejects unknown fields (such as "name").
func loginBody(email, password string) string {
	b, _ := json.Marshal(map[string]string{"email": email, "password": password})
	return string(b)
}

func register(r *gin.Engine, email, password string) int {
	return sendJSON(r, http.MethodPost, "/api/v1/auth/register", credentials(email, password)).Code
}

func loginToken(t *testing.T, r *gin.Engine, email, password string) string {
	t.Helper()
	w := sendJSON(r, http.MethodPost, "/api/v1/auth/login", loginBody(email, password))
	if w.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	data, _ := decode(t, w).Data.(map[string]any)
	token, _ := data["access_token"].(string)
	if token == "" {
		t.Fatalf("login response has no access_token: %s", w.Body.String())
	}
	return token
}

func TestRegister_Success(t *testing.T) {
	w := sendJSON(newTestRouter(), http.MethodPost, "/api/v1/auth/register",
		credentials("  Ada@Example.com ", "secret123"))

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	resp := decode(t, w)
	if resp.Message != "Account created successfully" {
		t.Fatalf("unexpected message: %s", resp.Message)
	}
	data, _ := resp.Data.(map[string]any)
	if data["email"] != "ada@example.com" { // trimmed and lowercased
		t.Fatalf("email not normalized: %v", data["email"])
	}
	// The password and its hash must never appear in a response.
	if strings.Contains(strings.ToLower(w.Body.String()), "password") {
		t.Fatalf("response leaks password data: %s", w.Body.String())
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	r := newTestRouter()
	if code := register(r, "ada@example.com", "secret123"); code != http.StatusCreated {
		t.Fatalf("first register: want 201, got %d", code)
	}
	w := sendJSON(r, http.MethodPost, "/api/v1/auth/register", credentials("ADA@example.com", "secret123"))
	expectError(t, w, http.StatusConflict, utils.CodeConflict)
}

func TestRegister_Validation(t *testing.T) {
	w := sendJSON(newTestRouter(), http.MethodPost, "/api/v1/auth/register",
		`{"name":"","email":"not-an-email","password":"short"}`)

	resp := expectError(t, w, http.StatusUnprocessableEntity, utils.CodeValidation)
	fields := detailFields(resp)
	for _, f := range []string{"name", "email", "password"} {
		if fields[f] == "" {
			t.Errorf("expected a validation message for %q, got %v", f, fields)
		}
	}
}

func TestRegister_WeakPassword(t *testing.T) {
	w := sendJSON(newTestRouter(), http.MethodPost, "/api/v1/auth/register",
		credentials("ada@example.com", "onlyletterspassword"))

	resp := expectError(t, w, http.StatusUnprocessableEntity, utils.CodeValidation)
	if detailFields(resp)["password"] != "password must contain at least one letter and one number" {
		t.Fatalf("unexpected details: %v", resp.Error.Details)
	}
}

func TestLogin_Success_ThenAccessProtectedRoute(t *testing.T) {
	r := newTestRouter()
	register(r, "ada@example.com", "secret123")

	w := sendJSON(r, http.MethodPost, "/api/v1/auth/login", loginBody("ada@example.com", "secret123"))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	resp := decode(t, w)
	if resp.Message != "Login successful" {
		t.Fatalf("unexpected message: %s", resp.Message)
	}
	data, _ := resp.Data.(map[string]any)
	if data["token_type"] != "Bearer" || data["expires_in"] != float64(3600) {
		t.Fatalf("unexpected token fields: %v", data)
	}

	token, _ := data["access_token"].(string)
	w = send(r, http.MethodGet, "/api/v1/books", "", "application/json", "Bearer "+token)
	if w.Code != http.StatusOK {
		t.Fatalf("books with a fresh token: want 200, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestLogin_WrongPasswordAndUnknownEmail_LookIdentical(t *testing.T) {
	r := newTestRouter()
	register(r, "ada@example.com", "secret123")

	wrongPassword := expectError(t,
		sendJSON(r, http.MethodPost, "/api/v1/auth/login", loginBody("ada@example.com", "wrong-pass1")),
		http.StatusUnauthorized, utils.CodeUnauthorized)
	unknownEmail := expectError(t,
		sendJSON(r, http.MethodPost, "/api/v1/auth/login", loginBody("nobody@example.com", "secret123")),
		http.StatusUnauthorized, utils.CodeUnauthorized)

	if wrongPassword.Error.Message != unknownEmail.Error.Message {
		t.Fatalf("responses must not reveal which emails exist: %q vs %q",
			wrongPassword.Error.Message, unknownEmail.Error.Message)
	}
}

func TestLogin_Validation(t *testing.T) {
	w := sendJSON(newTestRouter(), http.MethodPost, "/api/v1/auth/login", `{"email":"bad","password":""}`)
	expectError(t, w, http.StatusUnprocessableEntity, utils.CodeValidation)
}

func TestMe_ReturnsCurrentUser(t *testing.T) {
	r := newTestRouter()
	register(r, "ada@example.com", "secret123")
	token := loginToken(t, r, "ada@example.com", "secret123")

	w := send(r, http.MethodGet, "/api/v1/auth/me", "", "application/json", "Bearer "+token)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	data, _ := decode(t, w).Data.(map[string]any)
	if data["email"] != "ada@example.com" {
		t.Fatalf("unexpected profile: %v", data)
	}
}

func TestMe_RequiresAuth(t *testing.T) {
	expectError(t, sendJSON(newTestRouter(), http.MethodGet, "/api/v1/auth/me", ""),
		http.StatusUnauthorized, utils.CodeUnauthorized)
}

func TestMe_TokenForDeletedUser(t *testing.T) {
	// sendAuthed signs a valid token for user 1, but no such user exists in this fresh store.
	w := sendAuthed(t, newTestRouter(), http.MethodGet, "/api/v1/auth/me", "")
	expectError(t, w, http.StatusUnauthorized, utils.CodeUnauthorized)
}
