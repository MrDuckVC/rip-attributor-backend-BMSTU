package handler

import (
	"attributor/internal/app/auth"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"net/http"
	"strings"

	"attributor/internal/app/dto"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// RegisterUser godoc
// @ID registerAuthor
// @Summary Зарегистрировать автора
// @Tags Авторы
// @Produce json
// @Accept json
// @Param user body dto.RegisterUser true "Логин и пароль"
// @Success 201 {object} dto.User
// @Failure 400
// @Failure 500
// @Failure 409
// @Router /users [post]
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

// Login godoc
// @ID loginAuthor
// @Summary Войти и получить JWT
// @Tags Авторы
// @Produce json
// @Accept json
// @Param credentials body dto.LoginRequest true "Логин и пароль"
// @Success 200 {object} dto.LoginResponse
// @Failure 400
// @Failure 500
// @Failure 401
// @Router /users/login [post]
func (h *Handler) Login(ctx *gin.Context) {
	var data dto.LoginRequest
	if decodeJSON(ctx, &data, false) != nil || binding.Validator.ValidateStruct(data) != nil || strings.TrimSpace(data.Login) == "" || len([]byte(data.Password)) > 72 {
		ctx.Status(http.StatusBadRequest)
		return
	}
	user, err := h.Repository.GetUserByLogin(ctx.Request.Context(), data.Login)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ctx.Status(http.StatusUnauthorized)
		return
	}
	if err != nil {
		respondError(ctx, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(data.Password)) != nil {
		ctx.Status(http.StatusUnauthorized)
		return
	}
	token, ttl, err := h.Auth.Issue(user.ID)
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, dto.LoginResponse{AccessToken: token, TokenType: "Bearer", ExpiresIn: ttl})
}

// Logout godoc
// @ID logoutAuthor
// @Summary Выйти и отозвать текущий JWT
// @Tags Авторы
// @Produce json
// @Success 200
// @Failure 400
// @Failure 500
// @Security BearerAuth
// @Failure 401
// @Failure 503
// @Router /users/logout [post]
func (h *Handler) Logout(ctx *gin.Context) {
	if decodeJSON(ctx, &struct{}{}, true) != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	err := h.Auth.Logout(ctx.Request.Context(), ctx.GetString("jwt"), tokenClaims(ctx).ExpiresAt.Time)
	if errors.Is(err, auth.ErrUnavailable) {
		ctx.Status(http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.Status(http.StatusOK)
}
