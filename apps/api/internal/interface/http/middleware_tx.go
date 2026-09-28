package httpapi

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/domain/dto"
	"github.com/yurihartmann/rifa-stress-test/apps/api/internal/infra/database"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gorm"
)

func transactionMiddleware(db *gorm.DB, logger *slog.Logger, metrics RequestMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, dto.ErrorResponse{Code: dto.CodeInternal, Message: "internal error"})
			return
		}
		tx := db.WithContext(c.Request.Context()).Begin()
		if tx.Error != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, dto.ErrorResponse{Code: dto.CodeInternal, Message: "internal error"})
			return
		}

		original := c.Writer
		buffered := newBufferedWriter(original)
		c.Writer = buffered

		spanName := "db.transaction"
		switch {
		case c.Request.Method == http.MethodPost && strings.HasSuffix(c.FullPath(), "/orders"):
			spanName = "checkout"
		case c.Request.Method == http.MethodPost && c.FullPath() == "/v1/payments/webhooks":
			spanName = "confirm_payment"
		}
		ctx, span := tracer.Start(c.Request.Context(), spanName)
		ctx = database.WithTx(ctx, tx)
		c.Request = c.Request.WithContext(ctx)

		started := time.Now()
		outcome := "rollback"
		ended := false
		finish := func() {
			if ended {
				return
			}
			ended = true
			span.SetAttributes(attribute.String("tx.outcome", outcome))
			span.End()
			if metrics != nil && spanName == "checkout" {
				metrics.RecordReservation(c.Request.Context(), time.Since(started), outcome)
			}
		}

		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			_ = tx.Rollback()
			outcome = "rollback"
			c.Writer = original
			c.AbortWithStatusJSON(http.StatusInternalServerError, dto.ErrorResponse{Code: dto.CodeInternal, Message: "internal error"})
			if logger != nil {
				logger.ErrorContext(c.Request.Context(), "request panic", "panic", fmt.Sprint(recovered), "route", c.FullPath())
			}
			finish()
		}()

		c.Next()

		status := buffered.Status()
		if status >= 200 && status < 300 {
			if err := tx.Commit().Error; err != nil {
				_ = tx.Rollback()
				outcome = "rollback"
				c.Writer = original
				c.AbortWithStatusJSON(http.StatusInternalServerError, dto.ErrorResponse{Code: dto.CodeInternal, Message: "internal error"})
				if logger != nil {
					logger.ErrorContext(c.Request.Context(), "commit failed", "error", err.Error(), "route", c.FullPath())
				}
			} else {
				outcome = "commit"
				buffered.flush()
			}
		} else {
			_ = tx.Rollback()
			outcome = "rollback"
			buffered.flush()
		}
		finish()
	}
}

type bufferedWriter struct {
	gin.ResponseWriter
	body   bytes.Buffer
	status int
}

func newBufferedWriter(writer gin.ResponseWriter) *bufferedWriter {
	return &bufferedWriter{ResponseWriter: writer}
}

func (w *bufferedWriter) WriteHeader(code int) {
	if code > 0 {
		w.status = code
	}
}

func (w *bufferedWriter) WriteHeaderNow() {}

func (w *bufferedWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

func (w *bufferedWriter) WriteString(value string) (int, error) {
	return w.Write([]byte(value))
}

func (w *bufferedWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *bufferedWriter) Size() int { return w.body.Len() }

func (w *bufferedWriter) Written() bool { return false }

func (w *bufferedWriter) Flush() {}

func (w *bufferedWriter) flush() {
	w.ResponseWriter.WriteHeader(w.Status())
	if w.body.Len() == 0 {
		return
	}
	_, _ = w.ResponseWriter.Write(w.body.Bytes())
}
