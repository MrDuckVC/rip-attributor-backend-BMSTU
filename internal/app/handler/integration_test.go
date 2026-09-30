//go:build integration

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"attributor/internal/app/ds"
	"attributor/internal/app/dto"
	"attributor/internal/app/migration"
	"attributor/internal/app/repository"
	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestAPIIntegration(t *testing.T) {
	baseDSN := os.Getenv("LAB3_TEST_DSN")
	if baseDSN == "" {
		t.Skip("set LAB3_TEST_DSN to run against PostgreSQL and MinIO")
	}
	ctx := context.Background()
	admin, err := gorm.Open(postgres.Open(baseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("lab3_test_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	dsn := baseDSN + " search_path=" + schema
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	// Start with lab 2 columns and a record deleted using only the old flag.
	if err := db.AutoMigrate(&ds.User{}, &ds.Corpus{}, &ds.Like{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE corpora ADD COLUMN prep_percent real, ADD COLUMN conj_percent real, ADD COLUMN is_delete boolean DEFAULT false").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ds.User{ID: 1, Login: "test", Password: "legacy"}).Error; err != nil {
		t.Fatal(err)
	}
	legacy := ds.Corpus{CreatorID: 1, Author: "Legacy", Source: "Legacy", Status: ds.StatusPublished, DateCreate: time.Now()}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&legacy).Update("is_delete", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := migration.Apply(db); err != nil {
		t.Fatal(err)
	}
	if err := migration.Apply(db); err != nil {
		t.Fatalf("migration is not repeatable: %v", err)
	}
	if err := db.First(&legacy, legacy.ID).Error; err != nil || legacy.Status != ds.StatusDeleted {
		t.Fatal("legacy deletion lost")
	}
	for _, column := range []string{"prep_percent", "conj_percent", "is_delete"} {
		if db.Migrator().HasColumn(&ds.Corpus{}, column) {
			t.Fatalf("column remains: %s", column)
		}
	}
	bucket := strings.ReplaceAll(schema, "_", "-")
	t.Setenv("MINIO_BUCKET_NAME", bucket)
	rep, err := repository.New(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := rep.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}
	client, err := minio.New(os.Getenv("MINIO_ENDPOINT"), &minio.Options{Creds: credentials.NewStaticV4(os.Getenv("MINIO_ROOT_USER"), os.Getenv("MINIO_ROOT_PASSWORD"), ""), Secure: false})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for object := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			if object.Err != nil {
				t.Error(object.Err)
				continue
			}
			if err := client.RemoveObject(ctx, bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
				t.Error(err)
			}
		}
		if err := client.RemoveBucket(ctx, bucket); err != nil {
			t.Error(err)
		}
	})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(rep).RegisterHandler(router)
	request := func(method, path, body string, code int) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		router.ServeHTTP(response, req)
		if response.Code != code {
			t.Fatalf("%s %s: want %d, got %d %s", method, path, code, response.Code, response.Body.String())
		}
		return response
	}
	multipartRequest := func(source string, withMedia bool) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for name, value := range map[string]string{"author": "Автор", "source": source, "word_count": "200", "pron_percent": "8.5"} {
			if err := writer.WriteField(name, value); err != nil {
				t.Fatal(err)
			}
		}
		if withMedia {
			for name, filename := range map[string]string{"image": "puskin.png", "video": "puskin.mp4"} {
				file, err := os.Open(filepath.Join("../../../resources/media", filename))
				if err != nil {
					t.Fatal(err)
				}
				part, err := writer.CreateFormFile(name, filename)
				if err != nil {
					file.Close()
					t.Fatal(err)
				}
				_, err = io.Copy(part, file)
				file.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/api/corpora", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	decodeCorpus := func(response *httptest.ResponseRecorder) dto.Corpus {
		var corpus dto.Corpus
		if err := json.Unmarshal(response.Body.Bytes(), &corpus); err != nil {
			t.Fatal(err)
		}
		return corpus
	}
	request("GET", "/api/corpora/draft", "", 404)
	request("GET", "/api/corpora/feed", "", 404)
	empty := request("GET", "/api/corpora", "", 200)
	if !strings.Contains(empty.Body.String(), `"items":[]`) {
		t.Fatal("empty list must be []")
	}
	created := multipartRequest("Источник", true)
	if created.Code != 201 {
		t.Fatalf("creation: %d %s", created.Code, created.Body.String())
	}
	corpus := decodeCorpus(created)
	if corpus.IsOwner != 1 || corpus.PronPercent == nil || *corpus.PronPercent != 8.5 {
		t.Fatal("invalid corpus response")
	}
	var stored ds.Corpus
	if err := db.First(&stored, corpus.ID).Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range []*string{stored.ImageURL, stored.VideoURL} {
		if name == nil || strings.Contains(*name, "http") {
			t.Fatal("database must store names")
		}
		if _, err := client.StatObject(ctx, bucket, *name, minio.StatObjectOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	if len(corpus.ImageURL) == 0 || len(corpus.VideoURL) == 0 {
		t.Fatal("missing URLs")
	}
	for _, location := range []string{corpus.ImageURL, corpus.VideoURL} {
		response, err := http.Get(location)
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 {
			t.Fatalf("uploaded file URL: %d %v", response.StatusCode, err)
		}
	}
	if multipartRequest("Второй", false).Code != 409 {
		t.Fatal("duplicate draft accepted")
	}
	if draft := decodeCorpus(request("GET", "/api/corpora/draft", "", 200)); draft.ID != corpus.ID {
		t.Fatal("wrong draft")
	}
	publication := fmt.Sprintf("/api/corpora/%d/publication", corpus.ID)
	request("PUT", publication, `{"status":"черновик"}`, 400)
	request("PUT", publication, "", 200)
	request("PUT", publication, "", 409)
	request("GET", "/api/corpora/draft", "", 404)
	filtered := request("GET", "/api/corpora?query=201", "", 200)
	if !strings.Contains(filtered.Body.String(), `"items":[]`) {
		t.Fatal("filter failed")
	}
	request("GET", "/api/corpora?query=200", "", 200)
	feed := decodeCorpus(request("GET", "/api/corpora/feed", "", 200))
	if feed.ID != corpus.ID || feed.DateFinish == nil {
		t.Fatal("wrong feed")
	}
	request("GET", fmt.Sprintf("/api/corpora/feed?id=%d&next=true", corpus.ID), "", 200)
	likePath := fmt.Sprintf("/api/corpora/%d/like", corpus.ID)
	request("POST", likePath, `{"like":1}`, 200)
	request("POST", likePath, `{"like":1}`, 200)
	liked := decodeCorpus(request("GET", "/api/corpora/feed", "", 200))
	if liked.IsLiked != 1 || liked.LikesCount != 1 {
		t.Fatal("like duplicate or missing")
	}
	request("POST", likePath, `{"like":0}`, 200)
	request("POST", likePath, `{"like":0}`, 200)
	registration := request("POST", "/api/users", `{"login":"new_user","password":"test123"}`, 201)
	var user dto.User
	if err := json.Unmarshal(registration.Body.Bytes(), &user); err != nil {
		t.Fatal(err)
	}
	var storedUser ds.User
	if err := db.First(&storedUser, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte("test123")); err != nil {
		t.Fatal(err)
	}
	request("POST", "/api/users", `{"login":"new_user","password":"test123"}`, 409)
	foreign := ds.Corpus{CreatorID: user.ID, Author: "Чужой", Source: "Книга", Status: ds.StatusPublished, DateCreate: time.Now()}
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	request("DELETE", fmt.Sprintf("/api/corpora/%d", foreign.ID), "", 403)
	request("PUT", fmt.Sprintf("/api/corpora/%d/publication", foreign.ID), "", 403)
	nextFeed := decodeCorpus(request("GET", fmt.Sprintf("/api/corpora/feed?id=%d&next=true", corpus.ID), "", 200))
	if nextFeed.ID != foreign.ID || nextFeed.IsOwner != 0 {
		t.Fatal("next feed failed")
	}
	request("DELETE", fmt.Sprintf("/api/corpora/%d", corpus.ID), "", 200)
	request("DELETE", fmt.Sprintf("/api/corpora/%d", corpus.ID), "", 404)
	request("GET", fmt.Sprintf("/api/corpora/feed?id=%d", corpus.ID), "", 404)
	request("POST", likePath, `{"like":1}`, 404)
	if err := db.First(&stored, corpus.ID).Error; err != nil || stored.Status != ds.StatusDeleted {
		t.Fatal("soft delete failed")
	}
	// Uploads are removed when inserting the database row fails.
	if err := db.Exec(`CREATE FUNCTION reject_test_source() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.source = 'db_failure' THEN RAISE EXCEPTION 'test failure'; END IF; RETURN NEW; END $$`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TRIGGER reject_test_source BEFORE INSERT ON corpora FOR EACH ROW EXECUTE FUNCTION reject_test_source()").Error; err != nil {
		t.Fatal(err)
	}
	objectCount := func() int {
		count := 0
		for object := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			if object.Err != nil {
				t.Fatal(object.Err)
			}
			count++
		}
		return count
	}
	before := objectCount()
	if result := multipartRequest("db_failure", true); result.Code != 500 {
		t.Fatalf("db failure got %d", result.Code)
	}
	if objectCount() != before {
		t.Fatal("objects leaked after rollback")
	}
	request("GET", "/api/corpora/draft", "", 404)
	// MinIO error must not leave a database draft either.
	t.Setenv("MINIO_BUCKET_NAME", bucket+"-missing")
	unavailable, err := repository.New(dsn)
	if err != nil {
		t.Fatal(err)
	}
	failureRouter := gin.New()
	NewHandler(unavailable).RegisterHandler(failureRouter)
	original := router
	router = failureRouter
	if result := multipartRequest("minio_failure", true); result.Code != 500 {
		t.Fatalf("MinIO failure got %d", result.Code)
	}
	router = original
	t.Setenv("MINIO_BUCKET_NAME", bucket)
	request("GET", "/api/corpora/draft", "", 404)
	// Concurrent creations must produce exactly one draft.
	var group sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		group.Add(1)
		go func() { defer group.Done(); codes <- multipartRequest("Concurrent", false).Code }()
	}
	group.Wait()
	close(codes)
	successes, conflicts := 0, 0
	for code := range codes {
		if code == 201 {
			successes++
		}
		if code == 409 {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent results: %d successes, %d conflicts", successes, conflicts)
	}
}
