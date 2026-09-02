package handler

import (
	"net/http"
	"strconv"

	"attributor/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{
		Repository: r,
	}
}

const currentUserID = 1

// Плитка (Grid)
func (h *Handler) GetGrid(ctx *gin.Context) {
	var corpora []repository.AuthorCorpus
	var err error

	searchQuery := ctx.Query("query")

	if searchQuery == "" {
		allCorpora, _ := h.Repository.GetCorpora()
		for _, c := range allCorpora {
			if c.Status == "опубликован" {
				corpora = append(corpora, c)
			}
		}
	} else {
		minWords, errParse := strconv.Atoi(searchQuery)
		if errParse != nil {
			logrus.Warn("Некорректный запрос поиска: ", searchQuery)
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
		for _, userID := range c.Likes {
			if userID == currentUserID {
				isLiked = true
				break
			}
		}

		corporaData = append(corporaData, gin.H{
			"ID":          c.ID,
			"Author":      c.Author,
			"Source":      c.Source,
			"ImageURL":    c.ImageURL,
			"WordCount":   c.WordCount,
			"PrepPercent": c.PrepPercent,
			"LikesCount":  len(c.Likes),
			"IsLiked":     isLiked,
		})
	}

	ctx.HTML(http.StatusOK, "grid.html", gin.H{
		"profiles": corporaData, // оставляем ключи шаблона для совместимости с html
		"query":    searchQuery,
	})
}

// Лента (Feed)
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
	if err != nil {
		corpus, _ = h.Repository.GetCorpusByID(1)
	}

	isLiked := false
	for _, userID := range corpus.Likes {
		if userID == currentUserID {
			isLiked = true
			break
		}
	}

	ctx.HTML(http.StatusOK, "feed.html", gin.H{
		"corpus":     corpus, // Передаем как .corpus вместо .profile
		"likesCount": len(corpus.Likes),
		"isLiked":    isLiked,
	})
}

// Добавление (Draft)
func (h *Handler) GetDraft(ctx *gin.Context) {
	allCorpora, _ := h.Repository.GetCorpora()
	var draft repository.AuthorCorpus

	for _, c := range allCorpora {
		if c.Status == "черновик" {
			draft = c
			break
		}
	}

	ctx.HTML(http.StatusOK, "add.html", gin.H{
		"draft": draft,
	})
}
