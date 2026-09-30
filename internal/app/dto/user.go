package dto

type RegisterUser struct {
	Login    string `json:"login" binding:"required,max=25"`
	Password string `json:"password" binding:"required,max=72"`
}

type User struct {
	ID    uint   `json:"id"`
	Login string `json:"login"`
}
