package model

type GensparkLoginResponse struct {
	Status  int               `json:"status"`
	Message string            `json:"message"`
	Data    GensparkLoginData `json:"data"`
}

type GensparkLoginData struct {
	CogenID    string `json:"cogen_id"`
	CogenName  string `json:"cogen_name"`
	CogenEmail string `json:"cogen_email"`
	IsLogin    bool   `json:"is_login"`
}
