package handler

import (
	"bytes"
	"html/template"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"attributor/internal/app/ds"
	"github.com/gin-gonic/gin"
)

func TestNameTemplates(t *testing.T) {
	templates, err := template.ParseGlob("../../../templates/*.html")
	if err != nil {
		t.Fatal(err)
	}
	corpus := &ds.Corpus{ID: 5, Name: "Пушкин, Евгений Онегин"}
	cases := []struct {
		name      string
		data      gin.H
		wantTitle bool
	}{
		{"corpus_form.html", gin.H{"isDraftExists": false, "draft": (*ds.Corpus)(nil)}, false},
		{"corpus_form.html", gin.H{"isDraftExists": true, "draft": corpus}, true},
		{"corpus.html", gin.H{"corpus": corpus}, true},
		{"corpora.html", gin.H{"corpora": []gin.H{{"ID": corpus.ID, "Name": corpus.Name}}, "page": 1}, true},
	}
	for _, test := range cases {
		var output bytes.Buffer
		if err := templates.ExecuteTemplate(&output, test.name, test.data); err != nil {
			t.Fatal(err)
		}
		if test.wantTitle && !strings.Contains(output.String(), corpus.Name) {
			t.Fatalf("missing title in %s", test.name)
		}
		if strings.Contains(output.String(), `name="author"`) || strings.Contains(output.String(), `name="source"`) {
			t.Fatal("old title fields remain")
		}
		if test.name == "corpus_form.html" && !test.wantTitle && strings.Count(output.String(), `name="name"`) != 1 {
			t.Fatal("expected one name input")
		}
	}
}

func TestNameRejectsInvalidLength(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(nil)
	router.POST("/corpora/new", handler.CreateDraft)
	for _, name := range []string{"", strings.Repeat("Я", 151)} {
		body := url.Values{"name": {name}}.Encode()
		request := httptest.NewRequest("POST", "/corpora/new", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != 400 {
			t.Fatalf("got %d", response.Code)
		}
	}
}
