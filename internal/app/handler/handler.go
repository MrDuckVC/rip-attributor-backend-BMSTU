// Package handler обрабатывает HTTP-запросы приложения.
package handler

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"attributor/internal/app/ds"
	"attributor/internal/app/repository"
)

// Handler содержит зависимости обработчиков HTTP-запросов.
type Handler struct {
	Repository *repository.Repository
}

// NewHandler создаёт обработчики HTTP-запросов.
func NewHandler(r *repository.Repository) *Handler {
	return &Handler{
		Repository: r,
	}
}

// currentUserID задаёт идентификатор демонстрационного пользователя.
const currentUserID uint = 1

// roundFloat округляет процент до двух знаков после запятой.
func roundFloat(val *float64) float64 {
	if val == nil {
		return 0.0
	}
	return math.Round(*val*100) / 100
}

// RegisterHandler регистрирует маршруты приложения.
func (h *Handler) RegisterHandler(router *gin.Engine) {
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/corpora")
	})

	corporaGroup := router.Group("/corpora")
	{
		corporaGroup.GET("", h.GetCorpora)
		corporaGroup.GET("/new", h.GetDraft)
		corporaGroup.GET("/:id", h.GetCorpus)

		corporaGroup.POST("/new", h.CreateDraft)
		corporaGroup.POST("/publish", h.Publish)
		corporaGroup.POST("/delete", h.DeleteCorpus)
	}
}

// RegisterStatic подключает HTML-шаблоны и статические файлы.
func (h *Handler) RegisterStatic(router *gin.Engine) {
	router.LoadHTMLGlob("templates/*")
	router.Static("/static", "./resources")
}

// GetCorpora возвращает каталог опубликованных корпусов.
func (h *Handler) GetCorpora(ctx *gin.Context) {
	var corpora []ds.Corpus
	var err error

	page, pageErr := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if pageErr != nil || page < 1 || page > int(^uint(0)>>1)/6 {
		ctx.Status(http.StatusBadRequest)
		return
	}
	var count int64
	searchQuery := ctx.Query("query")

	if searchQuery == "" {
		corpora, count, err = h.Repository.GetCorpora(page, 6)
	} else {
		minWords, errParse := strconv.Atoi(searchQuery)
		if errParse != nil {
			logrus.Warn("Некорректный запрос поиска: ", searchQuery)
			corpora, count, err = h.Repository.GetCorpora(page, 6)
		} else {
			corpora, count, err = h.Repository.GetCorporaByWordCount(minWords, page, 6)
			if err != nil {
				logrus.Error(err)
			}
		}
	}

	if err != nil {
		logrus.Error(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	var corporaData []gin.H
	for _, c := range corpora {
		isLiked := false
		for _, like := range c.Likes {
			if like.UserID == currentUserID {
				isLiked = true
				break
			}
		}

		imageURL := ""
		if c.ImageURL != nil && *c.ImageURL != "" {
			imageURL = *c.ImageURL
		}

		corporaData = append(corporaData, gin.H{
			"ID":          c.ID,
			"Name":        c.Name,
			"ImageURL":    imageURL,
			"WordCount":   c.WordCount,
			"PrepPercent": roundFloat(c.PrepPercent),
			"LikesCount":  len(c.Likes),
			"IsLiked":     isLiked,
		})
	}

	ctx.HTML(http.StatusOK, "corpora.html", gin.H{
		"corpora":      corporaData,
		"query":        searchQuery,
		"page":         page,
		"previousPage": page - 1,
		"nextPage":     page + 1,
		"hasPrevious":  page > 1,
		"hasNext":      int64(page*6) < count,
	})
}

// GetCorpus показывает страницу корпуса или следующий корпус в ленте.
func (h *Handler) GetCorpus(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.String(http.StatusBadRequest, "Неверный ID")
		return
	}

	isNext := ctx.Query("next") == "true"

	var corpus *ds.Corpus
	if isNext {
		corpus, err = h.Repository.GetNextCorpus(id)
	} else {
		corpus, err = h.Repository.GetCorpusByID(id)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ctx.String(http.StatusNotFound, "Услуга не найдена или была удалена")
		return
	}
	if err != nil {
		logrus.Error(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	videoURL := ""
	if corpus.VideoURL != nil && *corpus.VideoURL != "" {
		videoURL = *corpus.VideoURL
	}

	if corpus.PrepPercent != nil {
		rounded := roundFloat(corpus.PrepPercent)
		corpus.PrepPercent = &rounded
	}
	if corpus.PronPercent != nil {
		rounded := roundFloat(corpus.PronPercent)
		corpus.PronPercent = &rounded
	}
	if corpus.ConjPercent != nil {
		rounded := roundFloat(corpus.ConjPercent)
		corpus.ConjPercent = &rounded
	}

	isLiked := false
	for _, like := range corpus.Likes {
		if like.UserID == currentUserID {
			isLiked = true
			break
		}
	}

	ctx.HTML(http.StatusOK, "corpus.html", gin.H{
		"corpus":     corpus,
		"likesCount": len(corpus.Likes),
		"isLiked":    isLiked,
		"videoURL":   videoURL,
	})
}

// DeleteCorpus обрабатывает запрос на удаление корпуса.
func (h *Handler) DeleteCorpus(ctx *gin.Context) {
	strId := ctx.PostForm("corpus_id")
	id, err := strconv.Atoi(strId)
	if err != nil {
		logrus.Error("Ошибка парсинга ID для удаления: ", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "неверный ID"})
		return
	}

	err = h.Repository.DeleteCorpus(id)
	if err != nil {
		logrus.Error("Ошибка при удалении корпуса: ", err)
	}

	ctx.Redirect(http.StatusFound, "/corpora")
}

// CreateDraft обрабатывает запрос на создание черновика.
func (h *Handler) CreateDraft(ctx *gin.Context) {
	name := strings.TrimSpace(ctx.PostForm("name"))
	if name == "" || utf8.RuneCountInString(name) > 150 {
		ctx.Status(http.StatusBadRequest)
		return
	}

	draft := ds.Corpus{
		CreatorID:   currentUserID,
		Name:        name,
		Description: ctx.PostForm("description"),
	}

	if err := h.Repository.CreateDraft(&draft); err != nil {
		logrus.Error(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	ctx.Redirect(http.StatusFound, "/corpora/new")
}

// Publish обрабатывает запрос на публикацию черновика.
func (h *Handler) Publish(ctx *gin.Context) {
	strId := ctx.PostForm("draft_id")
	draftId, _ := strconv.Atoi(strId)

	wordCount, _ := strconv.Atoi(ctx.PostForm("word_count"))
	pron, _ := strconv.ParseFloat(ctx.PostForm("pron_percent"), 64)

	if wordCount < 0 || pron < 0 || pron > 100 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}

	_ = h.Repository.PublishCorpus(draftId, wordCount, pron)
	ctx.Redirect(http.StatusFound, "/corpora")
}

// GetDraft обрабатывает запрос черновика текущего пользователя.
func (h *Handler) GetDraft(ctx *gin.Context) {
	draft, err := h.Repository.GetDraft(currentUserID)
	isDraftExists := err == nil
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.Error(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.HTML(http.StatusOK, "corpus_form.html", gin.H{
		"draft":         draft,
		"isDraftExists": isDraftExists,
	})
}
