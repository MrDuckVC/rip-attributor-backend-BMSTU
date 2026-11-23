package dto

// RegisterUser содержит логин и пароль для регистрации.
type RegisterUser struct {
	Login    string `json:"login" binding:"required,max=25"`
	Password string `json:"password" binding:"required,max=72"`
}

// User содержит публичные данные пользователя.
type User struct {
	ID    uint   `json:"id"`
	Login string `json:"login"`
}
