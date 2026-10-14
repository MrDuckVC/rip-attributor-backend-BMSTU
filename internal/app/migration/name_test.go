//go:build integration

package migration

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"attributor/internal/app/ds"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestLegacyCorpusNames(t *testing.T) {
	dsn := os.Getenv("LAB3_TEST_DSN")
	if dsn == "" {
		t.Skip("set LAB3_TEST_DSN for PostgreSQL integration checks")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, overflow := range []bool{false, true} {
		t.Run(fmt.Sprintf("overflow_%t", overflow), func(t *testing.T) {
			schema := fmt.Sprintf("lab3_names_%d", time.Now().UnixNano())
			if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
				t.Fatal(err)
			}
			defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
			db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&ds.User{}, &ds.Corpus{}, &ds.Like{}); err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&ds.User{ID: 1, Login: "test", Password: "legacy"}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("ALTER TABLE corpora RENAME COLUMN name TO author; ALTER TABLE corpora ALTER COLUMN author TYPE varchar(50); ALTER TABLE corpora ADD COLUMN source varchar(100) NOT NULL DEFAULT ''").Error; err != nil {
				t.Fatal(err)
			}
			author, source := "Пушкин", "Евгений Онегин"
			if overflow {
				author, source = strings.Repeat("А", 50), strings.Repeat("Б", 100)
			}
			row := map[string]interface{}{"id": 7, "creator_id": 1, "author": author, "source": source, "word_count": 200, "pron_percent": 8.5, "description": "Сохранилось", "status": ds.StatusPublished, "date_create": time.Now()}
			if err := db.Table("corpora").Create(row).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Omit("User", "Corpus").Create(&ds.Like{UserID: 1, CorpusID: 7}).Error; err != nil {
				t.Fatal(err)
			}
			err = Apply(db)
			if overflow {
				if err == nil || !strings.Contains(err.Error(), "IDs: [7]") {
					t.Fatalf("expected ID-bearing overflow error, got %v", err)
				}
				if !db.Migrator().HasColumn(&ds.Corpus{}, "author") || !db.Migrator().HasColumn(&ds.Corpus{}, "source") || db.Migrator().HasColumn(&ds.Corpus{}, "name") {
					t.Fatal("failed migration changed title columns")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var corpus ds.Corpus
			if err := db.First(&corpus, 7).Error; err != nil {
				t.Fatal(err)
			}
			if corpus.Name != author+", "+source || corpus.WordCount != 200 || corpus.Status != ds.StatusPublished || corpus.Description != "Сохранилось" {
				t.Fatalf("record changed: %+v", corpus)
			}
			var count int64
			if err := db.Model(&ds.Like{}).Where("corpus_id = 7").Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("like changed: %d %v", count, err)
			}
			if db.Migrator().HasColumn(&ds.Corpus{}, "author") || db.Migrator().HasColumn(&ds.Corpus{}, "source") {
				t.Fatal("old columns remain")
			}
			if err := Apply(db); err != nil {
				t.Fatal(err)
			}
			var again ds.Corpus
			if err := db.First(&again, 7).Error; err != nil || again.Name != corpus.Name {
				t.Fatalf("repeated migration changed name: %q %v", again.Name, err)
			}
			if err := db.Exec("UPDATE corpora SET name = ? WHERE id = 7", strings.Repeat("Я", 150)).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("UPDATE corpora SET name = ? WHERE id = 7", strings.Repeat("Я", 151)).Error; err == nil {
				t.Fatal("name accepted 151 characters")
			}
		})
	}
}
