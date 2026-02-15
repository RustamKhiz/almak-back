package models

type Door struct {
	ID       uint    `json:"id" gorm:"primaryKey"`
	OrderID  uint    `json:"order_id" gorm:"index;not null"`
	Type     string  `json:"type" gorm:"not null"`
	Model    string  `json:"model" gorm:"not null"`
	Price    float64 `json:"price" gorm:"not null"`
	Color    string  `json:"color" gorm:"not null"`
	Width    int     `json:"width" gorm:"not null"`
	Height   int     `json:"height" gorm:"not null"`
	LeafType string  `json:"leafType" gorm:"not null"`
	Count    int     `json:"count" gorm:"not null"`
}
