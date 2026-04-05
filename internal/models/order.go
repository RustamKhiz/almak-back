package models

import "time"

type Order struct {
	ID              uint         `json:"id" gorm:"primaryKey"`
	Customer        string       `json:"customer" gorm:"not null"`
	Phone           string       `json:"phone" gorm:"not null"`
	Date            string       `json:"date" gorm:"not null"`
	Price           float64      `json:"price" gorm:"not null"`
	Prepayment      float64      `json:"prepayment" gorm:"not null"`
	Discount        float64      `json:"discount" gorm:"not null;default:0"`
	NeedsDelivery   bool         `json:"needsDelivery" gorm:"not null;default:false"`
	DeliveryAddress string       `json:"deliveryAddress"`
	Comment         string       `json:"comment"`
	Status          string       `json:"status" gorm:"not null"`
	InteriorDoors   []InteriorDoor `json:"interiorDoors,omitempty" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	EntranceDoors   []EntranceDoor `json:"entranceDoors,omitempty" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	Moldings        []Molding      `json:"moldings,omitempty" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	Extensions      []Extension    `json:"extensions,omitempty" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	Capitals        []Capital      `json:"capitals,omitempty" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	Panelings       []Paneling     `json:"panelings,omitempty" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	CreatedAt       time.Time    `json:"created_at"`
}
