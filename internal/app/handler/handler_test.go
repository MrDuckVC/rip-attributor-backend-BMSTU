package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"attributor/internal/app/dto"
	"github.com/gin-gonic/gin"
)

func TestInvalidRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(nil).RegisterHandler(router)
	for _, test := range []struct{ method, path, body, contentType string }{
		{"GET", "/api/corpora?page=0", "", ""},
		{"GET", "/api/corpora?query=-1", "", ""},
		{"GET", "/api/corpora?query=2147483648", "", ""},
		{"GET", "/api/corpora?query=", "", ""},
		{"GET", "/api/corpora/feed?id=-1", "", ""},
		{"GET", "/api/corpora/feed?id=999999999999", "", ""},
		{"GET", "/api/corpora/feed?id=%22", "", ""},
		{"GET", "/api/corpora/feed?next=true", "", ""},
		{"PUT", "/api/corpora/1/publication", `{"status":"черновик"}`, "application/json"},
		{"DELETE", "/api/corpora/1", `{"creator_id":2}`, "application/json"},
		{"POST", "/api/corpora/1/like", `{"like":2}`, "application/json"},
		{"POST", "/api/corpora/1/like", `{}`, "application/json"},
		{"POST", "/api/corpora/1/like", `{"like":1,"user_id":2}`, "application/json"},
		{"POST", "/api/users", `{"login":"","password":"test"}`, "application/json"},
		{"POST", "/api/users", `{"login":"test","password":"test","is_moderator":true}`, "application/json"},
		{"POST", "/api/users", `{"login":"test","password":"test"} {}`, "application/json"},
		{"POST", "/api/corpora", `{"author":"test"}`, "application/json"},
	} {
		t.Run(test.method+test.path+test.body, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
			if test.contentType != "" {
				req.Header.Set("Content-Type", test.contentType)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != http.StatusBadRequest || response.Body.Len() != 0 {
				t.Fatalf("want empty 400, got %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestMultipartRejectsSystemFieldsAndFakeFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(nil).RegisterHandler(router)
	for _, field := range []string{"status", "id", "creator_id", "date_create", "image", "video", "fake_image", "word_count", "pron_percent"} {
		t.Run(field, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			_ = writer.WriteField("author", "Автор")
			_ = writer.WriteField("source", "Источник")
			if field == "fake_image" {
				part, _ := writer.CreateFormFile("image", "image.png")
				_, _ = part.Write([]byte("not an image"))
			} else {
				value := "1"
				if field == "word_count" {
					value = "2147483648"
				}
				if field == "pron_percent" {
					value = "101"
				}
				_ = writer.WriteField(field, value)
			}
			_ = writer.Close()
			req := httptest.NewRequest("POST", "/api/corpora", &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != 400 {
				t.Fatalf("got %d", response.Code)
			}
		})
	}
}

func TestEmptyCorpusSerializer(t *testing.T) {
	payload, err := json.Marshal(dto.Corpus{})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"id", "author", "source", "description", "word_count", "pron_percent", "image_url", "video_url", "creator", "is_owner", "is_liked", "likes_count", "date_create", "date_finish"} {
		if _, ok := result[name]; !ok {
			t.Fatalf("missing %s", name)
		}
	}
	for _, name := range []string{"status", "password", "prep_percent", "conj_percent", "is_delete"} {
		if _, ok := result[name]; ok {
			t.Fatalf("unexpected %s", name)
		}
	}
}

func TestAuthStubs(t *testing.T) {
	router := gin.New()
	NewHandler(nil).RegisterHandler(router)
	for _, path := range []string{"/api/users/login", "/api/users/logout"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("POST", path, nil))
		if response.Code != 200 || response.Body.Len() != 0 || len(response.Result().Cookies()) != 0 {
			t.Fatalf("stub returned %d %s", response.Code, response.Body.String())
		}
	}
}

func TestUnknownRouteReturnsEmpty404(t *testing.T) {
	router := gin.New()
	NewHandler(nil).RegisterHandler(router)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/api/unknown", nil))
	if response.Code != 404 || response.Body.Len() != 0 {
		t.Fatalf("got %d %s", response.Code, response.Body.String())
	}
}
