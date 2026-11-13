package handlers

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"golang_crud/auth"
	"golang_crud/models"
	"golang_crud/utils"
)

const (
	msgRegistered   = "Account created successfully"
	msgLoggedIn     = "Login successful"
	msgProfileFetch = "Profile retrieved successfully"
)

// UserStore is what the auth handlers need from the data layer.
type UserStore interface {
	Create(ctx context.Context, u *models.User) error
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByID(ctx context.Context, id int64) (*models.User, error)
}

// TokenIssuer creates access tokens (implemented by *auth.TokenManager).
type TokenIssuer interface {
	Issue(userID int64, email string) (token string, expiresIn time.Duration, err error)
}

type AuthHandler struct {
	users  UserStore
	tokens TokenIssuer
}

func NewAuthHandler(users UserStore, tokens TokenIssuer) *AuthHandler {
	return &AuthHandler{users: users, tokens: tokens}
}

// ---- request / response types ------------------------------------------

type registerRequest struct {
	Name     string `json:"name" binding:"required,notblank,max=100"`
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,min=8,maxbytes=72,strongpassword"`
}

// Normalize runs before validation. Emails are case-insensitive in practice,
// so we store them trimmed and lowercased. Passwords are never altered.
func (r *registerRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,maxbytes=72"`
}

func (r *loginRequest) Normalize() {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
}

type tokenResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int          `json:"expires_in"` // seconds
	User        *models.User `json:"user"`
}

// One shared message for "unknown email" and "wrong password" so the API never
// reveals which emails are registered.
func invalidCredentials() *utils.AppError {
	return utils.NewUnauthorized("Invalid email or password")
}

// ---- handlers ----------------------------------------------------------

// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Error(c, err)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		utils.Error(c, err) // not an AppError => logged, client gets a generic 500
		return
	}

	user := models.User{Name: req.Name, Email: req.Email, PasswordHash: hash}
	if err := h.users.Create(c.Request.Context(), &user); err != nil {
		if errors.Is(err, models.ErrEmailTaken) {
			utils.Error(c, utils.NewConflict("An account with this email already exists"))
			return
		}
		utils.Error(c, err)
		return
	}

	utils.Created(c, msgRegistered, user)
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Error(c, err)
		return
	}

	user, err := h.users.GetByEmail(c.Request.Context(), req.Email)
	switch {
	case errors.Is(err, models.ErrUserNotFound):
		auth.BurnPasswordCheck(req.Password) // keep response time similar to a wrong password
		utils.Error(c, invalidCredentials())
		return
	case err != nil:
		utils.Error(c, err)
		return
	}

	if err := auth.CheckPassword(user.PasswordHash, req.Password); err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			utils.Error(c, invalidCredentials())
			return
		}
		utils.Error(c, err)
		return
	}

	token, expiresIn, err := h.tokens.Issue(user.ID, user.Email)
	if err != nil {
		utils.Error(c, err)
		return
	}

	utils.OK(c, msgLoggedIn, tokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(expiresIn.Seconds()),
		User:        user,
	})
}

// GET /api/v1/auth/me  (requires a valid access token)
func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := utils.UserID(c)
	if !ok {
		utils.Error(c, utils.NewUnauthorized("Authentication required"))
		return
	}

	user, err := h.users.GetByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			// A valid token for an account that no longer exists.
			utils.Error(c, utils.NewUnauthorized("This account no longer exists"))
			return
		}
		utils.Error(c, err)
		return
	}

	utils.OK(c, msgProfileFetch, user)
}
