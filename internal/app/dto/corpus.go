// Package dto описывает структуры запросов и ответов API.
package dto

import "time"

// CreateCorpus содержит поля, которые клиент может задать при создании корпуса.
type CreateCorpus struct {
	Name string `form:"name" binding:"required,max=150"`
}

// PublishCorpus содержит описание и статистику для публикации корпуса. Указатели отличают нулевое значение от пропущенного поля.
type PublishCorpus struct {
	Description string   `json:"description" binding:"required"`
	WordCount   *int     `json:"word_count" binding:"required,gte=0,lte=2147483647"`
	PronPercent *float64 `json:"pron_percent" binding:"required,gte=0,lte=100"`
}

// LikeRequest содержит требуемое состояние лайка.
type LikeRequest struct {
	Like *int `json:"like" binding:"required,oneof=0 1"`
}

// Corpus описывает корпус в ответах каталога, ленты и операций с черновиком. Пустые поля сохраняются в JSON.
type Corpus struct {
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	WordCount   int        `json:"word_count"`
	PronPercent *float64   `json:"pron_percent"`
	ImageURL    string     `json:"image_url"`
	VideoURL    string     `json:"video_url"`
	IsOwner     int        `json:"is_owner"`
	IsLiked     int        `json:"is_liked"`
	LikesCount  int64      `json:"likes_count"`
	DateCreate  time.Time  `json:"date_create"`
	DateFinish  *time.Time `json:"date_finish"`
}

// CorpusList содержит страницу каталога и общее число корпусов.
type CorpusList struct {
	Items []Corpus `json:"items"`
	Total int64    `json:"total"`
	Page  int      `json:"page"`
}

// LikeResponse возвращает состояние лайка и общее число лайков корпуса.
type LikeResponse struct {
	IsLiked    int   `json:"is_liked"`
	LikesCount int64 `json:"likes_count"`
}
