package ds

import "time"

const (
	StatusDraft     = "черновик"
	StatusPublished = "опубликован"
	StatusDeleted   = "удален"
)

type Corpus struct {
	ID          uint       `gorm:"primaryKey;type:int"`
	CreatorID   uint       `gorm:"type:int;not null"`
	ImageURL    *string    `gorm:"type:varchar(255)"`
	VideoURL    *string    `gorm:"type:varchar(255)"`
	Name        string     `gorm:"type:varchar(150);not null"`
	WordCount   int        `gorm:"type:int;not null;default:0"`
	PronPercent *float64   `gorm:"type:real"`
	Description string     `gorm:"type:text"`
	Status      string     `gorm:"type:varchar(20);not null"`
	DateCreate  time.Time  `gorm:"type:timestamp;not null"`
	DateFinish  *time.Time `gorm:"type:timestamp"`

	Likes []Like `gorm:"foreignKey:CorpusID"`
}

func (Corpus) TableName() string {
	return "corpora"
}
