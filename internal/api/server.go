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

	r.GET("/grid", h.GetGrid)
	r.GET("/feed/:id", h.GetFeed)
	r.GET("/add", h.GetDraft)

	// Редирект на стартовую страницу
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/grid")
	})

	log.Println("Server is running on :8080")
	r.Run("0.0.0.0:8080")
}
