package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"attributor/internal/app/dto"
	"attributor/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func parseCreate(ctx *gin.Context) (dto.CreateCorpus, []repository.Upload, error) {
	var data dto.CreateCorpus
	mediaType, _, err := mime.ParseMediaType(ctx.GetHeader("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		return data, nil, errors.New("expected multipart")
	}
	// This is a memory threshold, not a file-size limit; larger files use temporary storage.
	if err := ctx.Request.ParseMultipartForm(2 << 20); err != nil {
		return data, nil, err
	}
	allowed := map[string]bool{"author": true, "source": true, "description": true, "word_count": true, "pron_percent": true}
	for field, values := range ctx.Request.MultipartForm.Value {
		if !allowed[field] || len(values) != 1 {
			return data, nil, errors.New("invalid form field")
		}
	}
	if err := ctx.ShouldBind(&data); err != nil {
		return data, nil, err
	}
	if strings.TrimSpace(data.Author) == "" || strings.TrimSpace(data.Source) == "" {
		return data, nil, errors.New("empty field")
	}
	if err := binding.Validator.ValidateStruct(data); err != nil {
		return data, nil, err
	}
	files := []repository.Upload{}
	for field, headers := range ctx.Request.MultipartForm.File {
		if (field != "image" && field != "video") || len(headers) != 1 {
			return data, nil, errors.New("invalid file field")
		}
		upload, err := validateUpload(headers[0], field)
		if err != nil {
			return data, nil, err
		}
		files = append(files, upload)
	}
	return data, files, nil
}

func validateUpload(header *multipart.FileHeader, field string) (repository.Upload, error) {
	upload := repository.Upload{Header: header}
	file, err := header.Open()
	if err != nil {
		return upload, err
	}
	defer file.Close()
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return upload, err
	}
	if n == 0 {
		return upload, errors.New("empty file")
	}
	contentType := http.DetectContentType(buffer[:n])
	extensions := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp", "video/mp4": ".mp4", "video/webm": ".webm"}
	extension, valid := extensions[contentType]
	if !valid || !strings.HasPrefix(contentType, field+"/") {
		return upload, errors.New("invalid media type")
	}
	name := make([]byte, 16)
	if _, err := rand.Read(name); err != nil {
		return upload, err
	}
	upload.Name = hex.EncodeToString(name) + extension
	upload.ContentType = contentType
	return upload, nil
}
