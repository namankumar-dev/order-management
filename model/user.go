package model

import "time"

type Role string

const (
	RoleAdmin    Role = "ADMIN"
	RoleCustomer Role = "CUSTOMER"
)

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Email        string    `gorm:"uniqueIndex;size:254;not null" json:"email"`
	PasswordHash string    `gorm:"size:100;not null" json:"-"`
	Role         Role      `gorm:"type:varchar(20);not null" json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Actor struct {
	UserID uint
	Role   Role
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,max=72"`
}

type TokenResponse struct {
	AccessToken string `json:"accessToken"`
	TokenType   string `json:"tokenType"`
	ExpiresIn   int64  `json:"expiresIn"`
}
