package models

import "time"

type Order struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Customer   string    `json:"customer" gorm:"not null"`
	Phone      string    `json:"phone" gorm:"not null"`
	Date       string    `json:"date" gorm:"not null"`
	Count      int       `json:"count" gorm:"not null"`
	Price      float64   `json:"price" gorm:"not null"`
	Prepayment float64   `json:"prepayment" gorm:"not null"`
	Comment    string    `json:"comment"`
	Status     string    `json:"status" gorm:"not null"`
	Doors      []Door    `json:"orders,omitempty" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	CreatedAt  time.Time `json:"created_at"`
}
