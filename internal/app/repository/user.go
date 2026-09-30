package repository

import (
	"context"

	"attributor/internal/app/ds"
	"golang.org/x/crypto/bcrypt"
)

func (r *Repository) RegisterUser(ctx context.Context, login, password string) (*ds.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &ds.User{Login: login, Password: string(hash), IsModerator: false}
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, normalizeError(err)
	}
	return user, nil
}
