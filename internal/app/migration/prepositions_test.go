//go:build integration

package migration

import (
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPrepositionMigration(t *testing.T) {
	dsn := os.Getenv("LAB3_TEST_DSN")
	if dsn == "" {
		t.Skip("set LAB3_TEST_DSN")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, columns := range []string{"pron_percent real", "prep_percent real", "pron_percent real, prep_percent real"} {
		t.Run(columns, func(t *testing.T) {
			schema := fmt.Sprintf("lab4_percent_%d", time.Now().UnixNano())
			if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
				t.Fatal(err)
			}
			defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
			db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("CREATE TABLE corpora (id integer PRIMARY KEY, " + columns + ")").Error; err != nil {
				t.Fatal(err)
			}
			values := "(1,8.5),(2,NULL)"
			if columns == "pron_percent real, prep_percent real" {
				values = "(1,8.5,NULL),(2,9,4)"
			}
			if err := db.Exec("INSERT INTO corpora VALUES " + values).Error; err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if err := migratePrepPercent(db); err != nil {
					t.Fatal(err)
				}
			}
			var result []struct {
				ID          int
				PrepPercent *float64
			}
			if err := db.Table("corpora").Order("id").Find(&result).Error; err != nil {
				t.Fatal(err)
			}
			if len(result) != 2 || result[0].PrepPercent == nil || *result[0].PrepPercent != 8.5 {
				t.Fatal("demonstration value lost")
			}
			if columns == "pron_percent real, prep_percent real" && (result[1].PrepPercent == nil || *result[1].PrepPercent != 4) {
				t.Fatal("existing prep_percent overwritten")
			}
			if db.Migrator().HasColumn("corpora", "pron_percent") {
				t.Fatal("old column remains")
			}
		})
	}
}
