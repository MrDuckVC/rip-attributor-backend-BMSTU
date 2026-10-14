package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"attributor/internal/app/ds"
	"attributor/internal/app/dsn"
)

func main() {
	_ = godotenv.Load()

	db, err := gorm.Open(postgres.Open(dsn.FromEnv()), &gorm.Config{})
	if err != nil {
		log.Panic("failed to connect database: ", err)
	}

	err = migrate(db)
	if err != nil {
		log.Panic("cant migrate db: ", err)
	}

	log.Println("Migration completed successfully!")
}

// migrate preserves the title previously displayed as "Author, Source".
func migrate(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable(&ds.Corpus{}) {
			hasAuthor := tx.Migrator().HasColumn(&ds.Corpus{}, "author")
			hasSource := tx.Migrator().HasColumn(&ds.Corpus{}, "source")
			hasName := tx.Migrator().HasColumn(&ds.Corpus{}, "name")
			if hasAuthor || hasSource {
				if !hasAuthor || !hasSource || hasName {
					return fmt.Errorf("unexpected corpus title columns: expected author and source without name")
				}
				var ids []uint
				if err := tx.Table("corpora").Where("char_length(concat_ws(', ', NULLIF(author, ''), NULLIF(source, ''))) > 150").Order("id").Pluck("id", &ids).Error; err != nil {
					return err
				}
				if len(ids) > 0 {
					return fmt.Errorf("corpus names exceed 150 characters, IDs: %v", ids)
				}
				if err := tx.Migrator().RenameColumn(&ds.Corpus{}, "author", "name"); err != nil {
					return err
				}
				if err := tx.Migrator().AlterColumn(&ds.Corpus{}, "Name"); err != nil {
					return err
				}
				if err := tx.Model(&ds.Corpus{}).Where("TRUE").Update("name", gorm.Expr("concat_ws(', ', NULLIF(name, ''), NULLIF(source, ''))")).Error; err != nil {
					return err
				}
				if err := tx.Migrator().DropColumn(&ds.Corpus{}, "source"); err != nil {
					return err
				}
			}
		}
		return tx.AutoMigrate(&ds.User{}, &ds.Corpus{}, &ds.Like{})
	})
}
