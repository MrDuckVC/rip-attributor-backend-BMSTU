package dto

type RegisterUser struct {
	Login    string `json:"login" binding:"required,max=25"`
	Password string `json:"password" binding:"required,max=72"`
}

type User struct {
	ID    uint   `json:"id"`
	Login string `json:"login"`
}

type LoginRequest struct {
	Login    string `json:"login" binding:"required,max=25"`
	Password string `json:"password" binding:"required,max=72"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}
