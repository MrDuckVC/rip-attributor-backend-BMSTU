// Package ds описывает модели хранения данных.
package ds

import "time"

// Статусы корпуса.
const (
	// StatusDraft обозначает черновик корпуса.
	StatusDraft = "черновик"
	// StatusPublished обозначает опубликованный корпус.
	StatusPublished = "опубликован"
	// StatusDeleted обозначает удалённый корпус.
	StatusDeleted = "удален"
)

// Corpus описывает корпус текстов автора.
type Corpus struct {
	ID          uint       `gorm:"primaryKey;type:int"`
	CreatorID   uint       `gorm:"type:int;not null"`
	ImageURL    *string    `gorm:"type:varchar(255)"`
	VideoURL    *string    `gorm:"type:varchar(255)"`
	Name        string     `gorm:"type:varchar(150);not null"`
	WordCount   int        `gorm:"type:int;not null;default:0"`
	PrepPercent *float64   `gorm:"type:real"`
	Description string     `gorm:"type:text"`
	Status      string     `gorm:"type:varchar(20);not null"`
	DateCreate  time.Time  `gorm:"type:timestamp;not null"`
	DateFinish  *time.Time `gorm:"type:timestamp"`

	Likes []Like `gorm:"foreignKey:CorpusID"`
}

// TableName задаёт имя таблицы корпусов в PostgreSQL.
func (Corpus) TableName() string {
	return "corpora"
}
