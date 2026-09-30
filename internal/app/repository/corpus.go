package repository

import (
	"context"
	"time"

	"attributor/internal/app/ds"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) GetCorpora(ctx context.Context, minWords *int, page int) ([]ds.Corpus, int64, error) {
	corpora := []ds.Corpus{}
	query := r.db.WithContext(ctx).Model(&ds.Corpus{}).Where("status = ?", ds.StatusPublished)
	if minWords != nil {
		query = query.Where("word_count >= ?", *minWords)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}
	err := query.Preload("Creator").Order("id ASC").Limit(6).Offset((page - 1) * 6).Find(&corpora).Error
	return corpora, count, err
}

func (r *Repository) GetFeed(ctx context.Context, id *uint, next bool) (*ds.Corpus, error) {
	corpus := &ds.Corpus{}
	query := r.db.WithContext(ctx).Preload("Creator").Where("status = ?", ds.StatusPublished)
	if id != nil && !next {
		return corpus, query.Where("id = ?", *id).First(corpus).Error
	}
	if id != nil {
		err := query.Where("id > ?", *id).Order("id ASC").First(corpus).Error
		if err == nil || err != gorm.ErrRecordNotFound {
			return corpus, err
		}
	}
	// First limits the main corpus query to one row, including wraparound.
	return corpus, r.db.WithContext(ctx).Preload("Creator").Where("status = ?", ds.StatusPublished).Order("id ASC").First(corpus).Error
}

func (r *Repository) GetDraft(ctx context.Context, userID uint) (*ds.Corpus, error) {
	corpus := &ds.Corpus{}
	err := r.db.WithContext(ctx).Preload("Creator").Where("creator_id = ? AND status = ?", userID, ds.StatusDraft).First(corpus).Error
	return corpus, err
}

func (r *Repository) CreateDraft(ctx context.Context, corpus *ds.Corpus, files []Upload) error {
	uploaded := []string{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize creations for this user; the partial unique index also guards the invariant.
		var user ds.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, corpus.CreatorID).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&ds.Corpus{}).Where("creator_id = ? AND status = ?", corpus.CreatorID, ds.StatusDraft).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return ErrConflict
		}
		corpus.Status = ds.StatusDraft
		corpus.DateCreate = time.Now()
		for _, file := range files {
			uploaded = append(uploaded, file.Name)
			if err := r.upload(ctx, file); err != nil {
				return err
			}
		}
		corpus.Creator = ds.User{} // Never create or update an associated user here.
		if err := tx.Omit("Creator", "Likes").Create(corpus).Error; err != nil {
			return err
		}
		corpus.Creator = user
		return nil
	})
	if err != nil {
		r.removeUploads(uploaded)
	}
	return normalizeError(err)
}

func (r *Repository) ownedCorpus(tx *gorm.DB, id, userID uint) (*ds.Corpus, error) {
	corpus := &ds.Corpus{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status <> ?", id, ds.StatusDeleted).First(corpus).Error; err != nil {
		return nil, err
	}
	if corpus.CreatorID != userID {
		return nil, ErrForbidden
	}
	return corpus, nil
}

func (r *Repository) PublishCorpus(ctx context.Context, id, userID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		corpus, err := r.ownedCorpus(tx, id, userID)
		if err != nil {
			return err
		}
		if corpus.Status != ds.StatusDraft {
			return ErrConflict
		}
		return tx.Model(corpus).Updates(map[string]interface{}{"status": ds.StatusPublished, "date_finish": time.Now()}).Error
	})
}

func (r *Repository) DeleteCorpus(ctx context.Context, id, userID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		corpus, err := r.ownedCorpus(tx, id, userID)
		if err != nil {
			return err
		}
		if corpus.Status != ds.StatusDraft && corpus.Status != ds.StatusPublished {
			return ErrConflict
		}
		return tx.Model(corpus).Update("status", ds.StatusDeleted).Error
	})
}

func (r *Repository) LikeStats(ctx context.Context, corpusID, userID uint) (int64, int, error) {
	var total, mine int64
	query := r.db.WithContext(ctx).Model(&ds.Like{}).Where("corpus_id = ?", corpusID)
	if err := query.Count(&total).Error; err != nil {
		return 0, 0, err
	}
	if err := query.Where("user_id = ?", userID).Count(&mine).Error; err != nil {
		return 0, 0, err
	}
	if mine > 0 {
		return total, 1, nil
	}
	return total, 0, nil
}

func (r *Repository) SetLike(ctx context.Context, corpusID, userID uint, value int) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var corpus ds.Corpus
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ?", corpusID, ds.StatusPublished).First(&corpus).Error; err != nil {
			return err
		}
		if value == 1 {
			like := ds.Like{UserID: userID, CorpusID: corpusID}
			if err := tx.Omit("User", "Corpus").Clauses(clause.OnConflict{DoNothing: true}).Create(&like).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Where("user_id = ? AND corpus_id = ?", userID, corpusID).Delete(&ds.Like{}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&ds.Like{}).Where("corpus_id = ?", corpusID).Count(&count).Error
	})
	return count, err
}
