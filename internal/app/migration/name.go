package migration

import (
	"fmt"

	"attributor/internal/app/ds"
	"gorm.io/gorm"
)

// migrateCorpusName also accepts a database already migrated by lab 2.
// The caller runs it in the same transaction as the remaining lab 3 migration.
func migrateCorpusName(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&ds.Corpus{}) {
		return nil
	}
	hasAuthor := tx.Migrator().HasColumn(&ds.Corpus{}, "author")
	hasSource := tx.Migrator().HasColumn(&ds.Corpus{}, "source")
	hasName := tx.Migrator().HasColumn(&ds.Corpus{}, "name")
	if !hasAuthor && !hasSource {
		return nil
	}
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
	return tx.Migrator().DropColumn(&ds.Corpus{}, "source")
}
