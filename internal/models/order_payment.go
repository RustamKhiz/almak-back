package models

import "time"

type OrderPayment struct {
	ID                  uint      `json:"id" gorm:"primaryKey"`
	OrderID             uint      `json:"orderId" gorm:"not null;index"`
	Amount              float64   `json:"amount" gorm:"not null"`
	Comment             string    `json:"comment"`
	ReversalOfPaymentID *uint     `json:"reversalOfPaymentId,omitempty"`
	ReversedByPaymentID *uint     `json:"reversedByPaymentId,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
}
