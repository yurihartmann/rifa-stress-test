package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/dto"
)

func adminAuth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		got, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok || !secretsEqual(got, token) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
				Code:    dto.CodeUnauthorized,
				Message: "unauthorized",
			})
			return
		}
		c.Next()
	}
}

func bearerToken(header string) (string, bool) {
	const prefix = "bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}

func secretsEqual(got, want string) bool {
	sumGot := sha256.Sum256([]byte(got))
	sumWant := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(sumGot[:], sumWant[:]) == 1
}

func webhookAuthorized(c *gin.Context, secret string) bool {
	return secretsEqual(c.GetHeader("X-Webhook-Secret"), secret)
}

func logInternal(logger *slog.Logger, c *gin.Context, err error) {
	if logger == nil || err == nil {
		return
	}
	logger.ErrorContext(c.Request.Context(), "request failed", "error", err.Error(), "route", c.FullPath())
}
