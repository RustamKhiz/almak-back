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

type EntranceDoor struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	OrderID     uint    `json:"order_id" gorm:"index;not null"`
	Kind        string  `json:"kind" gorm:"not null"`
	Model       string  `json:"model" gorm:"not null"`
	Width       int     `json:"width" gorm:"not null"`
	Height      int     `json:"height" gorm:"not null"`
	Color       string  `json:"color" gorm:"not null"`
	Painting    *string `json:"painting"`
	PanelColor  *string `json:"panelColor"`
	HasPeephole *bool   `json:"hasPeephole"`
	Count       int     `json:"count" gorm:"not null"`
	Price       float64 `json:"price" gorm:"not null"`
	Comment     string  `json:"comment"`
}

func (EntranceDoor) TableName() string {
	return "entrance_doors"
}
