package api

import (
	"log"

	"attributor/internal/app/handler"
	"attributor/internal/app/repository"

	"github.com/gin-gonic/gin"
)

func StartServer() {
	repo, _ := repository.NewRepository()
	h := handler.NewHandler(repo)

	r := gin.Default()

	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./resources")

	// Группа маршрутов /corpora
	corporaGroup := r.Group("/corpora")
	{
		corporaGroup.GET("/grid", h.GetGrid) // Плитка: /corpora/grid
		corporaGroup.GET("/add", h.GetDraft) // Добавление: /corpora/add
		corporaGroup.GET("/:id", h.GetFeed)  // Лента: /corpora/1
	}

	// Редирект на стартовую страницу
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/corpora/grid")
	})

	log.Println("Server is running on :8080")
	r.Run("0.0.0.0:8080")
}
