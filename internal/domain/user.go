package domain

import "time"

// User: bilet alan kisi. PasswordHash ASLA JSON'a cikmamali -> json:"-".
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// AuthResult: basarili kayit/giris sonrasi client'a donen sey.
type AuthResult struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
