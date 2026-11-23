// Package handler обрабатывает HTTP-запросы приложения.
package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"attributor/internal/app/auth"
)

// currentUser возвращает идентификатор пользователя из контекста запроса.
func currentUser(ctx *gin.Context) uint {
	id, _ := ctx.Get("user_id")
	value, _ := id.(uint)
	return value
}

// withAuth проверяет токен Bearer и записывает данные пользователя в контекст запроса.
func (h *Handler) withAuth(required bool) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		parts := strings.Fields(ctx.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || h.Auth == nil {
			if required {
				ctx.AbortWithStatus(http.StatusUnauthorized)
			}
			return
		}
		id, claims, err := h.Auth.Validate(ctx.Request.Context(), parts[1])
		if err != nil {
			if required {
				code := http.StatusUnauthorized
				if errors.Is(err, auth.ErrUnavailable) {
					code = http.StatusServiceUnavailable
				}
				ctx.AbortWithStatus(code)
			}
			return
		}
		ctx.Set("user_id", id)
		ctx.Set("jwt", parts[1])
		ctx.Set("jwt_claims", claims)
	}
}

// tokenClaims возвращает проверенные утверждения JWT из контекста запроса.
func tokenClaims(ctx *gin.Context) *jwt.RegisteredClaims {
	return ctx.MustGet("jwt_claims").(*jwt.RegisteredClaims)
}
