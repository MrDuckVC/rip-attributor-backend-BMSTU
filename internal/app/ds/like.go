package ds

type Like struct {
	ID       uint `gorm:"primaryKey;type:int"`
	UserID   uint `gorm:"type:int;not null"`
	CorpusID uint `gorm:"type:int;not null"`

	User   User   `gorm:"foreignKey:UserID"`
	Corpus Corpus `gorm:"foreignKey:CorpusID"`
}
