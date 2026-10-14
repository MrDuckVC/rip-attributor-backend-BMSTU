package dto

import "time"

// CreateCorpus contains only fields which the client may set.
type CreateCorpus struct {
	Name        string `form:"name" binding:"required,max=150"`
	Description string `form:"description"`
}

// PublishCorpus accepts only the two thematic fields. Pointers distinguish zero from omission.
type PublishCorpus struct {
	WordCount   *int     `json:"word_count" binding:"required,gte=0,lte=2147483647"`
	PronPercent *float64 `json:"pron_percent" binding:"required,gte=0,lte=100"`
}

type LikeRequest struct {
	Like *int `json:"like" binding:"required,oneof=0 1"`
}

// Corpus is shared by the catalogue, feed, draft and creation responses.
// No omitempty: empty fields are still present in JSON.
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

type CorpusList struct {
	Items []Corpus `json:"items"`
	Total int64    `json:"total"`
	Page  int      `json:"page"`
}

type LikeResponse struct {
	IsLiked    int   `json:"is_liked"`
	LikesCount int64 `json:"likes_count"`
}
