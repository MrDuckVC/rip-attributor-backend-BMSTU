package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"attributor/internal/app/auth"
	"attributor/internal/app/config"
	"attributor/internal/app/dsn"
	"attributor/internal/app/handler"
	"attributor/internal/app/redis"
	"attributor/internal/app/repository"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// @title API корпусов авторов
// @version 4.0
// @description Корпусы текстов потенциальных авторов: каталог, лента, публикация и лайки. Процент предлогов и количество слов задаются при публикации.
// @BasePath /api
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Введите Bearer и пробел, затем access_token из ответа POST /api/users/login.
func main() {
	if err := run(); err != nil {
		logrus.Fatal(err)
	}
}

func run() error {
	conf, err := config.NewConfig()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	rep, err := repository.New(dsn.FromEnv())
	if err != nil {
		return fmt.Errorf("PostgreSQL: %w", err)
	}
	defer rep.Close()
	startup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	blacklist, err := redis.New(startup, conf.RedisAddress, conf.RedisPassword)
	if err != nil {
		return fmt.Errorf("Redis: %w", err)
	}
	defer blacklist.Close()
	authentication, err := auth.New(conf.JWTSecret, conf.JWTIssuer, conf.JWTTTL, blacklist)
	if err != nil {
		return err
	}
	if err := rep.EnsureBucket(startup); err != nil {
		return fmt.Errorf("MinIO: %w", err)
	}
	router := gin.Default()
	handler.NewHandler(rep, authentication).RegisterHandler(router)
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", conf.ServiceHost, conf.ServicePort), Handler: router, ReadHeaderTimeout: 10 * time.Second}
	stopped, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	failed := make(chan error, 1)
	go func() { failed <- server.ListenAndServe() }()
	logrus.Infof("API: %s; Swagger: /swagger/index.html", server.Addr)
	select {
	case err := <-failed:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-stopped.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
