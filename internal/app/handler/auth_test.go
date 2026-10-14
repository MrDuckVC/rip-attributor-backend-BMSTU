package handler

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"attributor/internal/app/auth"
	blacklist "attributor/internal/app/redis"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
)

func TestOptionalAndRequiredAuthentication(t *testing.T) {
	r := miniredis.RunT(t)
	store, err := blacklist.New(context.Background(), r.Addr(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s, _ := auth.New("a-private-test-secret-with-at-least-32-bytes", "attributor", time.Hour, store)
	raw, _, _ := s.Issue(42)
	h := NewHandler(nil, s)
	router := gin.New()
	router.GET("/public", h.withAuth(false), func(c *gin.Context) { c.JSON(200, currentUser(c)) })
	router.GET("/private", h.withAuth(true), func(c *gin.Context) { c.JSON(200, currentUser(c)) })
	request := func(path, token string, code int, body string) {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		if token != "" {
			req.Header.Set("Authorization", token)
		}
		router.ServeHTTP(w, req)
		if w.Code != code || w.Body.String() != body {
			t.Fatalf("%s: %d %q", path, w.Code, w.Body.String())
		}
	}
	for _, token := range []string{"", "Bearer broken", "Basic abc", "Bearer"} {
		request("/public", token, 200, "0")
		request("/private", token, 401, "")
	}
	request("/public", "Bearer "+raw, 200, "42")
	request("/private", "Bearer "+raw, 200, "42")
	_, claims, _ := s.Validate(context.Background(), raw)
	if err := s.Logout(context.Background(), raw, claims.ExpiresAt.Time); err != nil {
		t.Fatal(err)
	}
	request("/public", "Bearer "+raw, 200, "0")
	request("/private", "Bearer "+raw, 401, "")
	second, _, _ := s.Issue(42)
	r.Close()
	request("/public", "Bearer "+second, 200, "0")
	request("/private", "Bearer "+second, 503, "")
}
