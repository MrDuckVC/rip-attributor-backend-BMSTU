package handler

import (
	"net/http"
	"strings"

	"attributor/internal/app/dto"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

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

// Lab 3 stubs: no sessions, cookies, tokens or current-user changes.
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

func (h *Handler) Logout(ctx *gin.Context) {
	if decodeJSON(ctx, &struct{}{}, true) != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	ctx.Status(http.StatusOK)
}
