package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	_ "attributor/docs/swagger"
	"attributor/internal/app/auth"
	"attributor/internal/app/ds"
	"attributor/internal/app/dto"
	"attributor/internal/app/repository"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Handler struct {
	Repository *repository.Repository
	Auth       *auth.Service
}

func NewHandler(r *repository.Repository, a *auth.Service) *Handler {
	return &Handler{Repository: r, Auth: a}
}

func (h *Handler) RegisterHandler(router *gin.Engine) {
	router.NoRoute(func(ctx *gin.Context) { ctx.AbortWithStatus(http.StatusNotFound) })
	api := router.Group("/api")
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.PersistAuthorization(false)))
	api.GET("/corpora", h.withAuth(false), h.GetCorpora)
	api.GET("/corpora/feed", h.withAuth(false), h.GetFeed)
	api.GET("/corpora/draft", h.withAuth(true), h.GetDraft)
	api.POST("/corpora", h.withAuth(true), h.CreateDraft)
	api.PUT("/corpora/:id/publication", h.withAuth(true), h.Publish)
	api.DELETE("/corpora/:id", h.withAuth(true), h.DeleteCorpus)
	api.POST("/corpora/:id/like", h.withAuth(true), h.SetLike)
	api.POST("/users", h.RegisterUser)
	api.POST("/users/login", h.Login)
	api.POST("/users/logout", h.withAuth(true), h.Logout)
}

func respondError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		ctx.Status(http.StatusNotFound)
	case errors.Is(err, repository.ErrForbidden):
		ctx.Status(http.StatusForbidden)
	case errors.Is(err, repository.ErrConflict):
		ctx.Status(http.StatusConflict)
	default:
		logrus.WithError(err).Error("request failed")
		ctx.Status(http.StatusInternalServerError)
	}
}

func parseID(value string) (uint, error) {
	id, err := strconv.ParseUint(value, 10, 31)
	if err != nil || id == 0 {
		return 0, errors.New("invalid id")
	}
	return uint(id), nil
}

// Strict request DTOs reject system fields as well as any other unknown fields.
func decodeJSON(ctx *gin.Context, value interface{}, allowEmpty bool) error {
	contentType := ctx.GetHeader("Content-Type")
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			return errors.New("expected JSON")
		}
	} else if !allowEmpty {
		return errors.New("expected JSON")
	}
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		if allowEmpty && errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	if err := decoder.Decode(new(interface{})); !errors.Is(err, io.EOF) {
		return errors.New("extra JSON value")
	}
	return nil
}

func (h *Handler) serializeCorpus(ctx *gin.Context, corpus *ds.Corpus) (dto.Corpus, error) {
	current := currentUser(ctx)
	image, err := h.Repository.MediaURL(ctx.Request.Context(), corpus.ImageURL)
	if err != nil {
		return dto.Corpus{}, err
	}
	video, err := h.Repository.MediaURL(ctx.Request.Context(), corpus.VideoURL)
	if err != nil {
		return dto.Corpus{}, err
	}
	count, liked, err := h.Repository.LikeStats(ctx.Request.Context(), corpus.ID, current)
	if err != nil {
		return dto.Corpus{}, err
	}
	owner := 0
	if corpus.CreatorID == current {
		owner = 1
	}
	return dto.Corpus{
		ID: corpus.ID, Name: corpus.Name, Description: corpus.Description,
		WordCount: corpus.WordCount, PrepPercent: corpus.PrepPercent, ImageURL: image, VideoURL: video,
		IsOwner: owner, IsLiked: liked, LikesCount: count,
		DateCreate: corpus.DateCreate, DateFinish: corpus.DateFinish,
	}, nil
}

// GetCorpora godoc
// @ID getCorpora
// @Summary Каталог опубликованных корпусов
// @Tags Корпусы
// @Produce json
// @Param query query int false "Минимальное количество слов" minimum(0) maximum(2147483647)
// @Param page query int false "Страница, по 6 корпусов" minimum(1) default(1)
// @Success 200 {object} dto.CorpusList
// @Failure 400
// @Failure 500
// @Router /corpora [get]
func (h *Handler) GetCorpora(ctx *gin.Context) {
	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 || page > int(^uint(0)>>1)/6 {
		ctx.Status(http.StatusBadRequest)
		return
	}
	var minWords *int
	if value, exists := ctx.GetQuery("query"); exists {
		minimum, err := strconv.Atoi(value)
		if err != nil || minimum < 0 || minimum > 2147483647 {
			ctx.Status(http.StatusBadRequest)
			return
		}
		minWords = &minimum
	} else if ctx.Request.URL.Query().Has("query") {
		ctx.Status(http.StatusBadRequest)
		return
	}
	corpora, total, err := h.Repository.GetCorpora(ctx.Request.Context(), minWords, page)
	if err != nil {
		respondError(ctx, err)
		return
	}
	result := dto.CorpusList{Items: []dto.Corpus{}, Total: total, Page: page}
	for _, corpus := range corpora {
		data, err := h.serializeCorpus(ctx, &corpus)
		if err != nil {
			respondError(ctx, err)
			return
		}
		result.Items = append(result.Items, data)
	}
	ctx.JSON(http.StatusOK, result)
}

// GetFeed godoc
// @ID getCorpusFeed
// @Summary Лента корпусов
// @Tags Корпусы
// @Produce json
// @Param id query int false "ID корпуса" minimum(1)
// @Param next query bool false "Следующий опубликованный корпус; нужен id"
// @Success 200 {object} dto.Corpus
// @Failure 400
// @Failure 500
// @Failure 404
// @Router /corpora/feed [get]
func (h *Handler) GetFeed(ctx *gin.Context) {
	var id *uint
	if ctx.Request.URL.Query().Has("id") {
		value, err := parseID(ctx.Query("id"))
		if err != nil {
			ctx.Status(http.StatusBadRequest)
			return
		}
		id = &value
	}
	next := false
	if ctx.Request.URL.Query().Has("next") {
		switch ctx.Query("next") {
		case "true":
			next = true
		case "false":
		default:
			ctx.Status(http.StatusBadRequest)
			return
		}
		if id == nil {
			ctx.Status(http.StatusBadRequest)
			return
		}
	}
	corpus, err := h.Repository.GetFeed(ctx.Request.Context(), id, next)
	if err != nil {
		respondError(ctx, err)
		return
	}
	data, err := h.serializeCorpus(ctx, corpus)
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, data)
}

// GetDraft godoc
// @ID getCorpusDraft
// @Summary Черновик текущего автора
// @Tags Корпусы
// @Produce json
// @Success 200 {object} dto.Corpus
// @Failure 400
// @Failure 500
// @Security BearerAuth
// @Failure 401
// @Failure 503
// @Failure 404
// @Router /corpora/draft [get]
func (h *Handler) GetDraft(ctx *gin.Context) {
	corpus, err := h.Repository.GetDraft(ctx.Request.Context(), currentUser(ctx))
	if err != nil {
		respondError(ctx, err)
		return
	}
	data, err := h.serializeCorpus(ctx, corpus)
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, data)
}

// CreateDraft godoc
// @ID createCorpus
// @Summary Добавить корпус с изображением и видео
// @Tags Корпусы
// @Produce json
// @Accept multipart/form-data
// @Param name formData string true "Название корпуса" maxlength(150)
// @Param description formData string false "Описание корпуса"
// @Param image formData file false "JPEG, PNG, GIF, WebP"
// @Param video formData file false "Короткое вертикальное MP4 или WebM"
// @Success 201 {object} dto.Corpus
// @Failure 400
// @Failure 500
// @Security BearerAuth
// @Failure 401
// @Failure 503
// @Failure 409
// @Router /corpora [post]
func (h *Handler) CreateDraft(ctx *gin.Context) {
	data, files, err := parseCreate(ctx)
	if ctx.Request.MultipartForm != nil {
		defer ctx.Request.MultipartForm.RemoveAll()
	}
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	corpus := &ds.Corpus{CreatorID: currentUser(ctx), Name: data.Name, Description: data.Description}
	for _, file := range files {
		name := file.Name
		if file.ContentType[:6] == "image/" {
			corpus.ImageURL = &name
		} else {
			corpus.VideoURL = &name
		}
	}
	if err := h.Repository.CreateDraft(ctx.Request.Context(), corpus, files); err != nil {
		respondError(ctx, err)
		return
	}
	result, err := h.serializeCorpus(ctx, corpus)
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, result)
}

// Publish godoc
// @ID publishCorpus
// @Summary Опубликовать свой черновик корпуса
// @Tags Корпусы
// @Produce json
// @Param id path int true "ID корпуса" minimum(1)
// @Accept json
// @Param publication body dto.PublishCorpus true "Количество слов и процент предлогов"
// @Success 200
// @Failure 400
// @Failure 500
// @Security BearerAuth
// @Failure 401
// @Failure 503
// @Failure 404
// @Failure 403
// @Failure 409
// @Router /corpora/{id}/publication [put]
func (h *Handler) Publish(ctx *gin.Context) {
	id, err := parseID(ctx.Param("id"))
	var data dto.PublishCorpus
	if err != nil || decodeJSON(ctx, &data, false) != nil || binding.Validator.ValidateStruct(&data) != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	if err := h.Repository.PublishCorpus(ctx.Request.Context(), id, currentUser(ctx), *data.WordCount, *data.PrepPercent); err != nil {
		respondError(ctx, err)
		return
	}
	ctx.Status(http.StatusOK)
}

// DeleteCorpus godoc
// @ID deleteCorpus
// @Summary Логически удалить свой корпус
// @Tags Корпусы
// @Produce json
// @Param id path int true "ID корпуса" minimum(1)
// @Success 200
// @Failure 400
// @Failure 500
// @Security BearerAuth
// @Failure 401
// @Failure 503
// @Failure 404
// @Failure 403
// @Router /corpora/{id} [delete]
func (h *Handler) DeleteCorpus(ctx *gin.Context) {
	id, err := parseID(ctx.Param("id"))
	if err != nil || decodeJSON(ctx, &struct{}{}, true) != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	if err := h.Repository.DeleteCorpus(ctx.Request.Context(), id, currentUser(ctx)); err != nil {
		respondError(ctx, err)
		return
	}
	ctx.Status(http.StatusOK)
}

// SetLike godoc
// @ID setCorpusLike
// @Summary Установить или снять лайк корпуса
// @Tags Корпусы
// @Produce json
// @Param id path int true "ID корпуса" minimum(1)
// @Accept json
// @Param like body dto.LikeRequest true "1 поставить, 0 снять"
// @Success 200 {object} dto.LikeResponse
// @Failure 400
// @Failure 500
// @Security BearerAuth
// @Failure 401
// @Failure 503
// @Failure 404
// @Router /corpora/{id}/like [post]
func (h *Handler) SetLike(ctx *gin.Context) {
	id, err := parseID(ctx.Param("id"))
	var data dto.LikeRequest
	if err != nil || decodeJSON(ctx, &data, false) != nil || data.Like == nil || (*data.Like != 0 && *data.Like != 1) {
		ctx.Status(http.StatusBadRequest)
		return
	}
	count, err := h.Repository.SetLike(ctx.Request.Context(), id, currentUser(ctx), *data.Like)
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, dto.LikeResponse{IsLiked: *data.Like, LikesCount: count})
}
