package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/dto"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/entity"
)

func writeError(c *gin.Context, err error) {
	status, code, message := mapError(err)
	c.JSON(status, dto.ErrorResponse{Code: code, Message: message})
}

func mapError(err error) (int, string, string) {
	switch {
	case errors.Is(err, entity.ErrValidation):
		return http.StatusBadRequest, dto.CodeValidationError, publicMessage(err, "invalid request")
	case errors.Is(err, entity.ErrUnauthorized):
		return http.StatusUnauthorized, dto.CodeUnauthorized, "unauthorized"
	case errors.Is(err, entity.ErrNotFound):
		return http.StatusNotFound, dto.CodeNotFound, "resource not found"
	case errors.Is(err, entity.ErrInsufficientTickets):
		return http.StatusConflict, dto.CodeInsufficientTickets, "not enough tickets"
	case errors.Is(err, entity.ErrRaffleClosed):
		return http.StatusConflict, dto.CodeRaffleClosed, "raffle is not open"
	case errors.Is(err, entity.ErrOrderExpired):
		return http.StatusConflict, dto.CodeOrderExpired, "order expired"
	default:
		return http.StatusInternalServerError, dto.CodeInternal, "internal error"
	}
}

func publicMessage(err error, fallback string) string {
	var domainErr *entity.DomainError
	if errors.As(err, &domainErr) && domainErr.Msg != "" {
		return domainErr.Msg
	}
	return fallback
}

func validation(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, dto.ErrorResponse{Code: dto.CodeValidationError, Message: message})
}
