package migration

import (
	"gorm.io/gorm"

	"attributor/internal/app/ds"
)

// migratePrepPercent переносит демонстрационный процент из pron_percent в prep_percent с сохранением строк.
func migratePrepPercent(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&ds.Corpus{}) || !tx.Migrator().HasColumn(&ds.Corpus{}, "pron_percent") {
		return nil
	}
	if !tx.Migrator().HasColumn(&ds.Corpus{}, "prep_percent") {
		return tx.Migrator().RenameColumn(&ds.Corpus{}, "pron_percent", "prep_percent")
	}
	if err := tx.Exec("UPDATE corpora SET prep_percent = pron_percent WHERE prep_percent IS NULL").Error; err != nil {
		return err
	}
	return tx.Migrator().DropColumn(&ds.Corpus{}, "pron_percent")
}
