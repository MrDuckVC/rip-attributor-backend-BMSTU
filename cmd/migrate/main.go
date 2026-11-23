// Команда migrate выполняет миграции базы данных.
package main

import (
	"log"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"attributor/internal/app/dsn"
	"attributor/internal/app/migration"
)

// main подключается к PostgreSQL и применяет миграции.
func main() {
	_ = godotenv.Load()
	db, err := gorm.Open(postgres.Open(dsn.FromEnv()), &gorm.Config{TranslateError: true})
	if err != nil {
		log.Fatal("failed to connect database: ", err)
	}
	if err := migration.Apply(db); err != nil {
		log.Fatal("cannot migrate database: ", err)
	}
	log.Println("Migration completed successfully!")
}
