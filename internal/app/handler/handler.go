package handler

import (
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

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
		c.Redirect(http.StatusFound, "/corpora/grid")
	})

	corporaGroup := router.Group("/corpora")
	{
		corporaGroup.GET("/grid", h.GetGrid)
		corporaGroup.GET("/add", h.GetDraft)
		corporaGroup.GET("/:id", h.GetFeed)

		corporaGroup.POST("/add", h.CreateDraft)
		corporaGroup.POST("/publish", h.Publish)
		corporaGroup.POST("/delete", h.DeleteCorpus)
	}
}

func (h *Handler) RegisterStatic(router *gin.Engine) {
	router.LoadHTMLGlob("templates/*")
	router.Static("/static", "./resources")
}

func (h *Handler) GetGrid(ctx *gin.Context) {
	var corpora []ds.Corpus
	var err error

	searchQuery := ctx.Query("query")

	if searchQuery == "" {
		corpora, _ = h.Repository.GetCorpora()
	} else {
		minWords, errParse := strconv.Atoi(searchQuery)
		if errParse != nil {
			logrus.Warn("Некорректный запрос поиска: ", searchQuery)
			corpora, _ = h.Repository.GetCorpora()
		} else {
			corpora, err = h.Repository.GetCorporaByWordCount(minWords)
			if err != nil {
				logrus.Error(err)
			}
		}
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

		imageURL := "http://localhost:9000/attributor-media/puskin.png"
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

	ctx.HTML(http.StatusOK, "grid.html", gin.H{
		"corpora": corporaData,
		"query":   searchQuery,
	})
}

func (h *Handler) GetFeed(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.String(http.StatusBadRequest, "Invalid ID")
		return
	}

	isNext := ctx.Query("next")
	if isNext == "true" {
		id = id + 1
	}

	corpus, err := h.Repository.GetCorpusByID(id)
	if err != nil || corpus == nil {
		corpus, _ = h.Repository.GetCorpusByID(1)
	}

	videoURL := "http://localhost:9000/attributor-media/puskin.mp4"
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

	ctx.HTML(http.StatusOK, "feed.html", gin.H{
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

	ctx.Redirect(http.StatusFound, "/corpora/grid")
}

func (h *Handler) CreateDraft(ctx *gin.Context) {
	author := ctx.PostForm("author")
	source := ctx.PostForm("source")

	draft := ds.Corpus{
		CreatorID: currentUserID,
		Author:    author,
		Source:    source,
	}

	h.Repository.CreateDraft(&draft)
	ctx.Redirect(http.StatusFound, "/corpora/add")
}

func (h *Handler) Publish(ctx *gin.Context) {
	strId := ctx.PostForm("draft_id")
	draftId, _ := strconv.Atoi(strId)

	wordCount, _ := strconv.Atoi(ctx.PostForm("word_count"))
	prep, _ := strconv.ParseFloat(ctx.PostForm("prep_percent"), 64)
	pron, _ := strconv.ParseFloat(ctx.PostForm("pron_percent"), 64)
	conj, _ := strconv.ParseFloat(ctx.PostForm("conj_percent"), 64)
	description := ctx.PostForm("description")

	if wordCount < 0 || prep < 0 || prep > 100 || pron < 0 || pron > 100 || conj < 0 || conj > 100 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}

	h.Repository.PublishCorpus(draftId, wordCount, prep, pron, conj, description)
	ctx.Redirect(http.StatusFound, "/corpora/grid")
}

func (h *Handler) GetDraft(ctx *gin.Context) {
	var draft ds.Corpus
	isDraftExists := false

	allCorpora, _ := h.Repository.GetCorpora()
	for _, c := range allCorpora {
		if c.Status == "черновик" && c.CreatorID == currentUserID {
			draft = c
			isDraftExists = true
			break
		}
	}

	ctx.HTML(http.StatusOK, "add.html", gin.H{
		"draft":         draft,
		"isDraftExists": isDraftExists,
	})
}
