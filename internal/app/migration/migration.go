package migration

import (
	"attributor/internal/app/currentuser"
	"attributor/internal/app/ds"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Apply migrates lab 2 records without recreating tables or deleting data rows.
func Apply(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable(&ds.Corpus{}) && tx.Migrator().HasColumn(&ds.Corpus{}, "is_delete") {
			if err := tx.Model(&ds.Corpus{}).Where("is_delete = ?", true).Update("status", ds.StatusDeleted).Error; err != nil {
				return err
			}
		}
		if err := tx.AutoMigrate(&ds.User{}, &ds.Corpus{}, &ds.Like{}); err != nil {
			return err
		}
		for _, column := range []string{"prep_percent", "conj_percent", "is_delete"} {
			if tx.Migrator().HasColumn(&ds.Corpus{}, column) {
				if err := tx.Migrator().DropColumn(&ds.Corpus{}, column); err != nil {
					return err
				}
			}
		}
		if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_one_draft_per_user ON corpora (creator_id) WHERE status = 'черновик'").Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&ds.User{}).Where("id = ?", currentuser.Get()).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			hash, err := bcrypt.GenerateFromPassword([]byte("test123"), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			user := ds.User{ID: currentuser.Get(), Login: "test", Password: string(hash)}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		}
		// Explicit insertion of ID=1 must not collide with the next registration.
		return tx.Exec("SELECT setval(pg_get_serial_sequence('users', 'id'), COALESCE((SELECT MAX(id) FROM users), 1), true)").Error
	})
}
