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
	var profiles []repository.TextProfile
	var err error

	searchQuery := ctx.Query("query")

	if searchQuery == "" {
		allProfiles, _ := h.Repository.GetProfiles()
		for _, p := range allProfiles {
			if p.Status == "опубликован" {
				profiles = append(profiles, p)
			}
		}
	} else {
		minWords, errParse := strconv.Atoi(searchQuery)
		if errParse != nil {
			logrus.Warn("Некорректный запрос поиска: ", searchQuery)
		} else {
			profiles, err = h.Repository.GetProfilesByWordCount(minWords)
			if err != nil {
				logrus.Error(err)
			}
		}
	}

	var profilesData []gin.H
	for _, p := range profiles {
		isLiked := false
		for _, userID := range p.Likes {
			if userID == currentUserID {
				isLiked = true
				break
			}
		}

		profilesData = append(profilesData, gin.H{
			"ID":         p.ID,
			"Author":     p.Author,
			"Source":     p.Source,
			"ImageURL":   p.ImageURL,
			"WordCount":  p.WordCount,
			"LikesCount": len(p.Likes),
			"IsLiked":    isLiked,
		})
	}

	ctx.HTML(http.StatusOK, "grid.html", gin.H{
		"profiles": profilesData,
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

	profile, err := h.Repository.GetProfileByID(id)
	if err != nil {
		profile, _ = h.Repository.GetProfileByID(1)
	}

	isLiked := false
	for _, userID := range profile.Likes {
		if userID == currentUserID {
			isLiked = true
			break
		}
	}

	ctx.HTML(http.StatusOK, "feed.html", gin.H{
		"profile":    profile,
		"likesCount": len(profile.Likes),
		"isLiked":    isLiked,
	})
}

// Добавление (Draft)
func (h *Handler) GetDraft(ctx *gin.Context) {
	allProfiles, _ := h.Repository.GetProfiles()
	var draft repository.TextProfile

	for _, p := range allProfiles {
		if p.Status == "черновик" {
			draft = p
			break
		}
	}

	ctx.HTML(http.StatusOK, "add.html", gin.H{
		"draft": draft,
	})
}
