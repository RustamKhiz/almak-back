package models

import "time"

type User struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Login     string    `json:"login" gorm:"uniqueIndex;not null"`
	Password  string    `json:"-" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
}
