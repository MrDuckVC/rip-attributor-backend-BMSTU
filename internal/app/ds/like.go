package ds

type Like struct {
	ID       uint `gorm:"primaryKey;type:int"`
	UserID   uint `gorm:"type:int;not null;uniqueIndex:idx_user_corpus"`
	CorpusID uint `gorm:"type:int;not null;uniqueIndex:idx_user_corpus"`

	User   User   `gorm:"foreignKey:UserID"`
	Corpus Corpus `gorm:"foreignKey:CorpusID"`
}
