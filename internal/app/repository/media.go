package repository

import (
	"context"
	"mime/multipart"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

type Upload struct {
	Header      *multipart.FileHeader
	Name        string
	ContentType string
}

func (r *Repository) upload(ctx context.Context, upload Upload) error {
	file, err := upload.Header.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = r.minio.PutObject(ctx, r.bucket, upload.Name, file, upload.Header.Size, minio.PutObjectOptions{ContentType: upload.ContentType})
	return err
}

func (r *Repository) removeUploads(names []string) {
	// Cleanup must still run if the request context was cancelled.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, name := range names {
		if err := r.minio.RemoveObject(ctx, r.bucket, name, minio.RemoveObjectOptions{}); err != nil {
			logrus.WithError(err).Error("cannot remove uploaded object")
		}
	}
}

func (r *Repository) MediaURL(ctx context.Context, name *string) (string, error) {
	if name == nil || *name == "" {
		return "", nil
	}
	// Existing lab 2 records may still reference files in their original buckets.
	if isLegacyURL(*name) {
		return *name, nil
	}
	location, err := r.minio.PresignedGetObject(ctx, r.bucket, *name, time.Hour, url.Values{})
	if err != nil {
		return "", err
	}
	return location.String(), nil
}
