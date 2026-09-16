package main

import (
	"log"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"attributor/internal/app/ds"
	"attributor/internal/app/dsn"
)

func main() {
	_ = godotenv.Load()

	db, err := gorm.Open(postgres.Open(dsn.FromEnv()), &gorm.Config{})
	if err != nil {
		log.Panic("failed to connect database: ", err)
	}

	err = db.AutoMigrate(
		&ds.User{},
		&ds.Corpus{},
		&ds.Like{},
	)
	if err != nil {
		log.Panic("cant migrate db: ", err)
	}

	log.Println("Migration completed successfully!")
}
