package handler

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"attributor/internal/app/ds"
	"attributor/internal/app/repository"
)

type Handler struct {
	Repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{
		Repository: r,
	}
}

const currentUserID uint = 1

func roundFloat(val *float64) float64 {
	if val == nil {
		return 0.0
	}
	return math.Round(*val*100) / 100
}

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

func (h *Handler) RegisterStatic(router *gin.Engine) {
	router.LoadHTMLGlob("templates/*")
	router.Static("/static", "./resources")
}

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
			"Author":      c.Author,
			"Source":      c.Source,
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

func (h *Handler) CreateDraft(ctx *gin.Context) {
	author := ctx.PostForm("author")
	source := ctx.PostForm("source")

	draft := ds.Corpus{
		CreatorID:   currentUserID,
		Author:      author,
		Source:      source,
		Description: ctx.PostForm("description"),
	}

	h.Repository.CreateDraft(&draft)
	ctx.Redirect(http.StatusFound, "/corpora/new")
}

func (h *Handler) Publish(ctx *gin.Context) {
	strId := ctx.PostForm("draft_id")
	draftId, _ := strconv.Atoi(strId)

	wordCount, _ := strconv.Atoi(ctx.PostForm("word_count"))
	pron, _ := strconv.ParseFloat(ctx.PostForm("pron_percent"), 64)

	if wordCount < 0 || pron < 0 || pron > 100 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}

	h.Repository.PublishCorpus(draftId, wordCount, pron)
	ctx.Redirect(http.StatusFound, "/corpora")
}

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
