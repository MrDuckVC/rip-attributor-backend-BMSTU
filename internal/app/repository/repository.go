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

var (
	ErrConflict  = errors.New("conflicting state")
	ErrForbidden = errors.New("another user's corpus")
)

type Repository struct {
	db     *gorm.DB
	minio  *minio.Client
	bucket string
}

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

func normalizeError(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrConflict
	}
	return err
}

func isLegacyURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}
