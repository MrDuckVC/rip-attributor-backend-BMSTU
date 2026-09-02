package repository

import (
	"fmt"
)

// AuthorCorpus - структура корпуса текста автора (наша "Услуга")
type AuthorCorpus struct {
	ID          int
	Author      string
	Source      string
	ImageURL    string
	VideoURL    string
	WordCount   int
	PrepPercent float64
	PronPercent float64
	ConjPercent float64
	Status      string // "опубликован", "черновик", "удален"
	Likes       []int  // Вложенные ID пользователей, поставивших лайк
}

type Repository struct{}

func NewRepository() (*Repository, error) {
	return &Repository{}, nil
}

func generateLikes(count int, likedByMe bool) []int {
	likes := make([]int, count)
	if likedByMe && count > 0 {
		likes[0] = 1
	} else if count > 0 {
		likes[0] = 999
	}
	return likes
}

// GetCorpora возвращает нашу мок-коллекцию корпусов
func (r *Repository) GetCorpora() ([]AuthorCorpus, error) {
	corpora := []AuthorCorpus{
		{
			ID:          1,
			Author:      "А. С. Пушкин",
			Source:      "«Капитанская дочка»",
			ImageURL:    "http://localhost:9000/attributor-media/puskin.png",
			VideoURL:    "http://localhost:9000/attributor-media/puskin.mp4",
			WordCount:   45200,
			PrepPercent: 8.42,
			PronPercent: 10.15,
			ConjPercent: 5.13,
			Status:      "опубликован",
			Likes:       generateLikes(141, true),
		},
		{
			ID:          2,
			Author:      "Ф. М. Достоевский",
			Source:      "«Преступление и наказание»",
			ImageURL:    "http://localhost:9000/attributor-media/dostoievski.png",
			VideoURL:    "http://localhost:9000/attributor-media/dostoievski.mp4",
			WordCount:   58700,
			PrepPercent: 9.10,
			PronPercent: 11.20,
			ConjPercent: 6.05,
			Status:      "опубликован",
			Likes:       generateLikes(115, false),
		},
		{
			ID:          3,
			Author:      "И. С. Тургенев",
			Source:      "«Отцы и дети»",
			ImageURL:    "http://localhost:9000/attributor-media/turgenev.png",
			VideoURL:    "http://localhost:9000/attributor-media/turgenev.mp4",
			WordCount:   54300,
			PrepPercent: 8.90,
			PronPercent: 10.50,
			ConjPercent: 5.80,
			Status:      "опубликован",
			Likes:       generateLikes(64, true),
		},
		{
			ID:          4,
			Author:      "Л. Н. Толстой",
			Source:      "«Война и мир (Том 1)»",
			ImageURL:    "http://localhost:9000/attributor-media/tolstoi.png",
			VideoURL:    "http://localhost:9000/attributor-media/tolstoi.mp4",
			WordCount:   120400,
			PrepPercent: 9.50,
			PronPercent: 10.80,
			ConjPercent: 6.20,
			Status:      "опубликован",
			Likes:       generateLikes(82, false),
		},
		// Пустой черновик
		{
			ID:          5,
			Author:      "Черновик",
			Source:      "Блокнот",
			ImageURL:    "",
			VideoURL:    "",
			WordCount:   8,
			PrepPercent: 99,
			PronPercent: 1,
			ConjPercent: 0,
			Status:      "черновик",
			Likes:       []int{},
		},
	}

	return corpora, nil
}

// GetCorpusByID
func (r *Repository) GetCorpusByID(id int) (AuthorCorpus, error) {
	corpora, _ := r.GetCorpora()
	for _, c := range corpora {
		if c.ID == id {
			return c, nil
		}
	}
	return AuthorCorpus{}, fmt.Errorf("корпус не найден")
}

// GetCorporaByWordCount
func (r *Repository) GetCorporaByWordCount(minWords int) ([]AuthorCorpus, error) {
	corpora, _ := r.GetCorpora()
	var result []AuthorCorpus
	for _, c := range corpora {
		if c.Status == "опубликован" && c.WordCount >= minWords {
			result = append(result, c)
		}
	}
	return result, nil
}
