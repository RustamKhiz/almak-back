package models

type InteriorDoor struct {
	ID       uint    `json:"id" gorm:"primaryKey"`
	OrderID  uint    `json:"order_id" gorm:"index;not null"`
	Model    string  `json:"model" gorm:"not null"`
	Price    float64 `json:"price" gorm:"not null"`
	Width    int     `json:"width" gorm:"not null"`
	Width2   *int    `json:"width2"`
	Height   int     `json:"height" gorm:"not null"`
	HasGlass bool    `json:"hasGlass" gorm:"not null;default:false"`
	LeafType string  `json:"leafType" gorm:"not null"`
	Count    int     `json:"count" gorm:"not null"`
	Covering string  `json:"covering" gorm:"not null;default:PVC"`
	Comment  string  `json:"comment"`
}

func (InteriorDoor) TableName() string {
	return "interior_doors"
}
