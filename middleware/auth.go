package middleware

import (
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"golang_crud/auth"
	"golang_crud/utils"
)

// TokenVerifier is implemented by *auth.TokenManager.
type TokenVerifier interface {
	Parse(token string) (*auth.Claims, error)
}

// Auth protects routes: it requires "Authorization: Bearer <access_token>", verifies the
// token, and stores the user's ID in the request context (read it with utils.UserID).
// On any problem it responds 401 and the handler never runs.
func Auth(verifier TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if header == "" {
			utils.Error(c, utils.NewUnauthorized("Authentication required: send an Authorization header"))
			return
		}

		scheme, token, found := strings.Cut(header, " ")
		token = strings.TrimSpace(token)
		if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
			utils.Error(c, utils.NewUnauthorized("Authorization header must be in the format: Bearer <access_token>"))
			return
		}

		claims, err := verifier.Parse(token)
		if err != nil {
			if errors.Is(err, auth.ErrTokenExpired) {
				utils.Error(c, utils.NewTokenExpired())
				return
			}
			// The reason stays in the debug log only; clients just get "invalid".
			slog.Debug("rejected access token", "request_id", utils.RequestID(c), "reason", err)
			utils.Error(c, utils.NewUnauthorized("Invalid access token"))
			return
		}

		userID, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil || userID < 1 {
			utils.Error(c, utils.NewUnauthorized("Invalid access token"))
			return
		}

		c.Set(utils.UserIDKey, userID)
		c.Next()
	}
}
