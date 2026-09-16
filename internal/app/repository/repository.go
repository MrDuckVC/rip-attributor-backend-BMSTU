package repository

import (
	"attributor/internal/app/ds"
	"database/sql"
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

func (r *Repository) GetCorpora() ([]ds.Corpus, error) {
	var corpora []ds.Corpus
	err := r.db.Preload("Likes").Where("is_delete = false").Find(&corpora).Error
	if err != nil {
		return nil, err
	}
	return corpora, nil
}

func (r *Repository) GetCorporaByWordCount(minWords int) ([]ds.Corpus, error) {
	var corpora []ds.Corpus
	err := r.db.Preload("Likes").Where("status = ? AND is_delete = false AND word_count >= ?", "опубликован", minWords).Find(&corpora).Error
	if err != nil {
		return nil, err
	}
	return corpora, nil
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
	query := `SELECT id, creator_id, is_delete, image_url, video_url, author, source, word_count, prep_percent, pron_percent, conj_percent, description, status, date_create, date_finish
			  FROM corpora WHERE id = $1 AND is_delete = false`

	row := r.db.Raw(query, id).Row()
	corpus := &ds.Corpus{}

	err := row.Scan(
		&corpus.ID, &corpus.CreatorID, &corpus.IsDelete, &corpus.ImageURL, &corpus.VideoURL,
		&corpus.Author, &corpus.Source, &corpus.WordCount,
		&corpus.PrepPercent, &corpus.PronPercent, &corpus.ConjPercent, &corpus.Description, &corpus.Status,
		&corpus.DateCreate, &corpus.DateFinish,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	r.db.Model(corpus).Association("Likes").Find(&corpus.Likes)

	return corpus, nil
}

func (r *Repository) CreateDraft(corpus *ds.Corpus) error {
	corpus.Status = "черновик"
	corpus.DateCreate = time.Now()
	return r.db.Create(corpus).Error
}

func (r *Repository) PublishCorpus(id int, wordCount int, prep, pron, conj float64, description string) error {
	now := time.Now()
	return r.db.Model(&ds.Corpus{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       "опубликован",
		"word_count":   wordCount,
		"prep_percent": prep,
		"pron_percent": pron,
		"conj_percent": conj,
		"description":  description,
		"date_finish":  &now,
	}).Error
}

func (r *Repository) DeleteCorpus(id int) error {
	query := "UPDATE corpora SET is_delete = true, status = 'удален' WHERE id = ?"
	return r.db.Exec(query, id).Error
}
