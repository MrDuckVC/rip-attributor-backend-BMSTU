package repository

import (
	"attributor/internal/app/ds"
	"errors"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(dsn string) (*Repository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return &Repository{
		db: db,
	}, nil
}

func (r *Repository) GetCorpora(page, pageSize int) ([]ds.Corpus, int64, error) {
	var corpora []ds.Corpus
	var count int64
	query := r.db.Model(&ds.Corpus{}).Where("status = ? AND is_delete = false", "опубликован")
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}
	err := query.Preload("Likes").Order("id ASC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&corpora).Error
	if err != nil {
		return nil, 0, err
	}
	return corpora, count, nil
}

func (r *Repository) GetCorporaByWordCount(minWords, page, pageSize int) ([]ds.Corpus, int64, error) {
	var corpora []ds.Corpus
	var count int64
	query := r.db.Model(&ds.Corpus{}).Where("status = ? AND is_delete = false AND word_count >= ?", "опубликован", minWords)
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}
	err := query.Preload("Likes").Order("id ASC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&corpora).Error
	if err != nil {
		return nil, 0, err
	}
	return corpora, count, nil
}

func (r *Repository) ToggleLike(userID, corpusID uint) error {
	var like ds.Like
	result := r.db.Where("user_id = ? AND corpus_id = ?", userID, corpusID).First(&like)

	if result.Error == nil {
		return r.db.Delete(&like).Error
	}

	newLike := ds.Like{UserID: userID, CorpusID: corpusID}
	return r.db.Create(&newLike).Error
}

func (r *Repository) GetCorpusByID(id int) (*ds.Corpus, error) {
	corpus := &ds.Corpus{}
	err := r.db.Preload("Likes").Where("id = ? AND status = ? AND is_delete = false", id, "опубликован").First(corpus).Error
	return corpus, err
}

func (r *Repository) GetNextCorpus(id int) (*ds.Corpus, error) {
	corpus := &ds.Corpus{}
	err := r.db.Preload("Likes").Where("id > ? AND status = ? AND is_delete = false", id, "опубликован").Order("id ASC").First(corpus).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = r.db.Preload("Likes").Where("status = ? AND is_delete = false", "опубликован").Order("id ASC").First(corpus).Error
	}
	return corpus, err
}

func (r *Repository) GetDraft(userID uint) (*ds.Corpus, error) {
	draft := &ds.Corpus{}
	err := r.db.Where("creator_id = ? AND status = ? AND is_delete = false", userID, "черновик").First(draft).Error
	return draft, err
}

func (r *Repository) CreateDraft(corpus *ds.Corpus) error {
	corpus.Status = "черновик"
	corpus.DateCreate = time.Now()
	return r.db.Create(corpus).Error
}

func (r *Repository) PublishCorpus(id int, wordCount int, pron float64) error {
	now := time.Now()
	return r.db.Model(&ds.Corpus{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       "опубликован",
		"word_count":   wordCount,
		"pron_percent": pron,
		"date_finish":  &now,
	}).Error
}

func (r *Repository) DeleteCorpus(id int) error {
	query := "UPDATE corpora SET is_delete = true, status = 'удален' WHERE id = ?"
	return r.db.Exec(query, id).Error
}
