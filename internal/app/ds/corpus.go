package ds

import "time"

type Corpus struct {
	ID          uint       `gorm:"primaryKey;type:int"`
	CreatorID   uint       `gorm:"type:int;not null"`
	IsDelete    bool       `gorm:"type:boolean;not null;default:false"`
	ImageURL    *string    `gorm:"type:varchar(255)"`
	VideoURL    *string    `gorm:"type:varchar(255)"`
	Author      string     `gorm:"type:varchar(50);not null"`
	Source      string     `gorm:"type:varchar(100);not null"`
	WordCount   int        `gorm:"type:int;not null;default:0"`
	PrepPercent *float64   `gorm:"type:real"`
	PronPercent *float64   `gorm:"type:real"`
	ConjPercent *float64   `gorm:"type:real"`
	Description string     `gorm:"type:text"`
	Status      string     `gorm:"type:varchar(20);not null"`
	DateCreate  time.Time  `gorm:"type:timestamp;not null"`
	DateFinish  *time.Time `gorm:"type:timestamp"`

	Creator User   `gorm:"foreignKey:CreatorID"`
	Likes   []Like `gorm:"foreignKey:CorpusID"`
}

func (Corpus) TableName() string {
	return "corpora"
}
