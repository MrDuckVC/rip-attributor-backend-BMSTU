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

// LoginRequest содержит учётные данные для входа.
type LoginRequest struct {
	Login    string `json:"login" binding:"required,max=25"`
	Password string `json:"password" binding:"required,max=72"`
}

// LoginResponse содержит токен доступа и срок его действия.
type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}
