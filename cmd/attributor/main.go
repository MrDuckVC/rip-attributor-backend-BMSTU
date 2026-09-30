package main

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"attributor/internal/app/config"
	"attributor/internal/app/dsn"
	"attributor/internal/app/handler"
	"attributor/internal/app/repository"
)

func main() {
	router := gin.Default()

	conf, err := config.NewConfig()
	if err != nil {
		logrus.Fatalf("error loading config: %v", err)
	}

	postgresString := dsn.FromEnv()
	rep, errRep := repository.New(postgresString)
	if errRep != nil {
		logrus.Fatalf("error initializing repository: %v", errRep)
	}

	hand := handler.NewHandler(rep)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := rep.EnsureBucket(ctx); err != nil {
		logrus.Fatalf("error initializing MinIO bucket: %v", err)
	}
	hand.RegisterHandler(router)

	serverAddress := fmt.Sprintf("%s:%d", conf.ServiceHost, conf.ServicePort)
	logrus.Infof("Server starting on %s", serverAddress)

	if err := router.Run(serverAddress); err != nil {
		logrus.Fatal(err)
	}
}
