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

	"attributor/internal/app/auth"
	"attributor/internal/app/ds"
	"attributor/internal/app/dto"
	"attributor/internal/app/migration"
	blacklist "attributor/internal/app/redis"
	"attributor/internal/app/repository"
	"github.com/alicebob/miniredis/v2"
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
	if err := db.Exec("ALTER TABLE corpora RENAME COLUMN prep_percent TO pron_percent; ALTER TABLE corpora ADD COLUMN conj_percent real, ADD COLUMN is_delete boolean DEFAULT false").Error; err != nil {
		t.Fatal(err)
	}
	seedHash, _ := bcrypt.GenerateFromPassword([]byte("test123"), bcrypt.DefaultCost)
	if err := db.Create(&ds.User{ID: 1, Login: "test", Password: string(seedHash)}).Error; err != nil {
		t.Fatal(err)
	}
	legacy := ds.Corpus{CreatorID: 1, Name: "Legacy, Legacy", Status: ds.StatusPublished, DateCreate: time.Now()}
	if err := db.Omit("PrepPercent").Create(&legacy).Error; err != nil {
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
	for _, column := range []string{"pron_percent", "conj_percent", "is_delete"} {
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
	redisAddress := os.Getenv("LAB4_TEST_REDIS_ADDR")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisAddress == "" {
		redisAddress = miniredis.RunT(t).Addr()
		redisPassword = ""
	}
	store, err := blacklist.New(ctx, redisAddress, redisPassword)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	authentication, err := auth.New("integration-private-secret-of-at-least-32-bytes", "attributor", time.Hour, store)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := authentication.Issue(1)
	if err != nil {
		t.Fatal(err)
	}
	NewHandler(rep, authentication).RegisterHandler(router)
	request := func(method, path, body string, code int) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		router.ServeHTTP(response, req)
		if response.Code != code {
			t.Fatalf("%s %s: want %d, got %d %s", method, path, code, response.Code, response.Body.String())
		}
		return response
	}
	multipartRequest := func(name string, withMedia bool) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for name, value := range map[string]string{"name": name} {
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
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	decodeCorpus := func(response *httptest.ResponseRecorder) dto.Corpus {
		var corpus dto.Corpus
		if err := json.Unmarshal(response.Body.Bytes(), &corpus); err != nil {
			t.Fatal(err)
		}
		var fields map[string]interface{}
		if err := json.Unmarshal(response.Body.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		for key, value := range fields {
			if key == "creator" || key == "creator_id" {
				t.Fatalf("unexpected creator field: %s", key)
			}
			if _, nested := value.(map[string]interface{}); nested {
				t.Fatalf("nested corpus field: %s", key)
			}
		}
		return corpus
	}
	request("POST", "/api/users/login", `{"login":"test","password":"wrong"}`, 401)
	request("POST", "/api/users/login", `{"login":"absent","password":"test123"}`, 401)
	var login dto.LoginResponse
	if err := json.Unmarshal(request("POST", "/api/users/login", `{"login":"test","password":"test123"}`, 200).Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	if login.TokenType != "Bearer" || login.ExpiresIn != 3600 || login.AccessToken == "" {
		t.Fatal("invalid login response")
	}
	token = login.AccessToken
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
	if corpus.Description != "" || corpus.IsOwner != 1 || corpus.PrepPercent != nil || corpus.WordCount != 0 {
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
	request("PUT", publication, `{"description":"Описание корпуса","word_count":200}`, 400)
	request("PUT", publication, `{"description":"Описание корпуса","word_count":200,"prep_percent":101}`, 400)
	if unchanged := decodeCorpus(request("GET", "/api/corpora/draft", "", 200)); unchanged.Description != "" || unchanged.WordCount != 0 || unchanged.PrepPercent != nil || unchanged.DateFinish != nil {
		t.Fatal("invalid publication changed draft")
	}
	request("PUT", publication, `{"word_count":200,"prep_percent":8.5}`, 400)
	request("PUT", publication, `{"description":" \t\n","word_count":200,"prep_percent":8.5}`, 400)
	request("PUT", publication, `{"description":"Описание корпуса","word_count":200,"prep_percent":8.5}`, 200)
	if err := db.First(&stored, corpus.ID).Error; err != nil || stored.Description != "Описание корпуса" || stored.Status != ds.StatusPublished || stored.WordCount != 200 || stored.PrepPercent == nil || *stored.PrepPercent != 8.5 {
		t.Fatalf("publication fields not saved atomically: %+v, %v", stored, err)
	}
	request("PUT", publication, `{"description":"Описание корпуса","word_count":0,"prep_percent":0}`, 409)
	request("GET", "/api/corpora/draft", "", 404)
	filtered := request("GET", "/api/corpora?query=201", "", 200)
	if !strings.Contains(filtered.Body.String(), `"items":[]`) {
		t.Fatal("filter failed")
	}
	request("GET", "/api/corpora?query=200", "", 200)
	feed := decodeCorpus(request("GET", "/api/corpora/feed", "", 200))
	if feed.ID != corpus.ID || feed.DateFinish == nil || feed.WordCount != 200 || feed.PrepPercent == nil || *feed.PrepPercent != 8.5 {
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
	foreign := ds.Corpus{ID: corpus.ID + 4, CreatorID: user.ID, Name: "Чужой, Книга", Status: ds.StatusPublished, DateCreate: time.Now()}
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("SELECT setval(pg_get_serial_sequence('corpora','id'), (SELECT MAX(id) FROM corpora), true)").Error; err != nil {
		t.Fatal(err)
	}
	request("DELETE", fmt.Sprintf("/api/corpora/%d", foreign.ID), "", 403)
	request("PUT", fmt.Sprintf("/api/corpora/%d/publication", foreign.ID), `{"description":"Описание корпуса","word_count":0,"prep_percent":0}`, 403)
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
	if err := db.Exec(`CREATE FUNCTION reject_test_name() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.name = 'db_failure' THEN RAISE EXCEPTION 'test failure'; END IF; RETURN NEW; END $$`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TRIGGER reject_test_name BEFORE INSERT ON corpora FOR EACH ROW EXECUTE FUNCTION reject_test_name()").Error; err != nil {
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
	NewHandler(unavailable, authentication).RegisterHandler(failureRouter)
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
	zeroDraft := decodeCorpus(request("GET", "/api/corpora/draft", "", 200))
	request("PUT", fmt.Sprintf("/api/corpora/%d/publication", zeroDraft.ID), `{"description":"Описание корпуса","word_count":0,"prep_percent":0}`, 200)
	zeroPublished := decodeCorpus(request("GET", fmt.Sprintf("/api/corpora/feed?id=%d", zeroDraft.ID), "", 200))
	if zeroPublished.Description != "Описание корпуса" || zeroPublished.WordCount != 0 || zeroPublished.PrepPercent == nil || *zeroPublished.PrepPercent != 0 || zeroPublished.DateFinish == nil {
		t.Fatal("zero publication values not saved")
	}

	ownerToken := token
	token = ""
	request("PUT", fmt.Sprintf("/api/corpora/%d/publication", zeroDraft.ID), `{"description":"Описание корпуса","word_count":1,"prep_percent":1}`, 401)
	for _, guestToken := range []string{"", "broken"} {
		token = guestToken
		guest := decodeCorpus(request("GET", fmt.Sprintf("/api/corpora/feed?id=%d", zeroDraft.ID), "", 200))
		if guest.IsOwner != 0 || guest.IsLiked != 0 {
			t.Fatal("guest personalized")
		}
	}
	token = ownerToken
	request("POST", "/api/users/logout", "", 200)
	request("GET", "/api/corpora/draft", "", 401)
	guest := decodeCorpus(request("GET", fmt.Sprintf("/api/corpora/feed?id=%d", zeroDraft.ID), "", 200))
	if guest.IsOwner != 0 {
		t.Fatal("revoked token still personalized")
	}
	restartedRouter := gin.New()
	NewHandler(rep, authentication).RegisterHandler(restartedRouter)
	router = restartedRouter
	request("GET", "/api/corpora/draft", "", 401)
	request("POST", "/api/users/login", `{"login":"new_user","password":"test123"}`, 200)
	var secondLogin dto.LoginResponse
	if err := json.Unmarshal(request("POST", "/api/users/login", `{"login":"new_user","password":"test123"}`, 200).Body.Bytes(), &secondLogin); err != nil {
		t.Fatal(err)
	}
	token = secondLogin.AccessToken
	request("GET", "/api/corpora/draft", "", 404)
	secondCreated := multipartRequest("Корпус второго автора", false)
	if secondCreated.Code != 201 {
		t.Fatalf("second user: %d", secondCreated.Code)
	}
	secondDraft := decodeCorpus(secondCreated)
	var secondStored ds.Corpus
	if err := db.First(&secondStored, secondDraft.ID).Error; err != nil || secondStored.CreatorID != user.ID {
		t.Fatal("wrong authenticated creator")
	}
	request("DELETE", fmt.Sprintf("/api/corpora/%d", zeroDraft.ID), "", 403)
	request("PUT", fmt.Sprintf("/api/corpora/%d/publication", secondDraft.ID), `{"description":"Описание корпуса","word_count":20,"prep_percent":5}`, 200)
	secondLikePath := fmt.Sprintf("/api/corpora/%d/like", zeroDraft.ID)
	request("POST", secondLikePath, `{"like":1}`, 200)
	secondFeed := decodeCorpus(request("GET", fmt.Sprintf("/api/corpora/feed?id=%d", zeroDraft.ID), "", 200))
	if secondFeed.IsOwner != 0 || secondFeed.IsLiked != 1 {
		t.Fatal("wrong second-user personalization")
	}
	token, _, _ = authentication.Issue(1)
	firstFeed := decodeCorpus(request("GET", fmt.Sprintf("/api/corpora/feed?id=%d", zeroDraft.ID), "", 200))
	if firstFeed.IsOwner != 1 || firstFeed.IsLiked != 0 {
		t.Fatal("identities leaked between requests")
	}
	request("GET", "/api/corpora/draft", "", 404)

}
