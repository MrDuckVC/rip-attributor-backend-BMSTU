package repository

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Ошибки пакета.
var (
	// ErrConflict обозначает конфликт с текущим состоянием данных.
	ErrConflict = errors.New("conflicting state")
	// ErrForbidden обозначает попытку изменить корпус другого пользователя.
	ErrForbidden = errors.New("another user's corpus")
)

// Repository предоставляет доступ к данным приложения.
type Repository struct {
	db     *gorm.DB
	minio  *minio.Client
	bucket string
}

// New открывает подключение к хранилищам приложения.
func New(dsn string) (*Repository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, err
	}
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9000"
	}
	secure := false
	if value := os.Getenv("MINIO_USE_SSL"); value != "" {
		secure, err = strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("MINIO_ROOT_USER"), os.Getenv("MINIO_ROOT_PASSWORD"), ""),
		Secure: secure,
	})
	if err != nil {
		return nil, err
	}
	bucket := os.Getenv("MINIO_BUCKET_NAME")
	if bucket == "" {
		bucket = "corpora"
	}
	return &Repository{db: db, minio: client, bucket: bucket}, nil
}

// EnsureBucket проверяет наличие бакета MinIO и создаёт его при необходимости.
func (r *Repository) EnsureBucket(ctx context.Context) error {
	exists, err := r.minio.BucketExists(ctx, r.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if err = r.minio.MakeBucket(ctx, r.bucket, minio.MakeBucketOptions{}); err != nil {
		// Another server may have created the bucket concurrently.
		if exists, checkErr := r.minio.BucketExists(ctx, r.bucket); checkErr == nil && exists {
			return nil
		}
		return err
	}
	return nil
}

// normalizeError преобразует ошибку конфликта PostgreSQL в ошибку репозитория.
func normalizeError(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrConflict
	}
	return err
}

// isLegacyURL определяет, хранится ли медиафайл по готовой HTTP-ссылке.
func isLegacyURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

// Close закрывает подключение к PostgreSQL.
func (r *Repository) Close() error {
	db, err := r.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}
