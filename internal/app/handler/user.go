package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"attributor/internal/app/dto"
)

// RegisterUser проверяет данные регистрации и возвращает созданного пользователя.
func (h *Handler) RegisterUser(ctx *gin.Context) {
	var data dto.RegisterUser
	if decodeJSON(ctx, &data, false) != nil || binding.Validator.ValidateStruct(data) != nil || strings.TrimSpace(data.Login) == "" || len([]byte(data.Password)) > 72 {
		ctx.Status(http.StatusBadRequest)
		return
	}
	user, err := h.Repository.RegisterUser(ctx.Request.Context(), data.Login, data.Password)
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, dto.User{ID: user.ID, Login: user.Login})
}

// Login проверяет запрос входа в демонстрационном режиме.
func (h *Handler) Login(ctx *gin.Context) {
	var data struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if decodeJSON(ctx, &data, true) != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	ctx.Status(http.StatusOK)
}

// Logout обрабатывает демонстрационный запрос выхода.
func (h *Handler) Logout(ctx *gin.Context) {
	if decodeJSON(ctx, &struct{}{}, true) != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	ctx.Status(http.StatusOK)
}
