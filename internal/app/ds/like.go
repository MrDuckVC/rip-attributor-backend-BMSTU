package ds

// Like связывает пользователя с корпусом, которому он поставил лайк.
type Like struct {
	ID       uint `gorm:"primaryKey;type:int"`
	UserID   uint `gorm:"type:int;not null;uniqueIndex:idx_user_corpus"`
	CorpusID uint `gorm:"type:int;not null;uniqueIndex:idx_user_corpus"`

	User   User   `gorm:"foreignKey:UserID"`
	Corpus Corpus `gorm:"foreignKey:CorpusID"`
}
