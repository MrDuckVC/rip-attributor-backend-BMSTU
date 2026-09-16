package dsn

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func FromEnv() string {
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")
	_ = godotenv.Load("../../.env")

	host := strings.TrimSpace(os.Getenv("DB_HOST"))
	if host == "" {
		host = "localhost"
	}
	port := strings.TrimSpace(os.Getenv("DB_PORT"))
	if port == "" {
		port = "5432"
	}
	user := strings.TrimSpace(os.Getenv("DB_USER"))
	pass := strings.TrimSpace(os.Getenv("DB_PASS"))
	dbname := strings.TrimSpace(os.Getenv("DB_NAME"))

	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, pass, dbname)
}
