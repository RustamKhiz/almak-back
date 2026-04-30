package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type InteriorDoor struct {
	ID           uint     `json:"id" gorm:"primaryKey"`
	OrderID      uint     `json:"order_id" gorm:"index;not null"`
	Model        string   `json:"model" gorm:"not null"`
	Color        string   `json:"color" gorm:"not null"`
	Price        float64  `json:"price" gorm:"not null"`
	Price2       *float64 `json:"price2"`
	Width        int      `json:"width" gorm:"not null"`
	Width2       *int     `json:"width2"`
	Height       int      `json:"height" gorm:"not null"`
	Height2      *int     `json:"height2"`
	HasGlass     bool     `json:"hasGlass" gorm:"not null;default:false"`
	GlassComment string   `json:"glassComment"`
	LeafType     string   `json:"leafType" gorm:"not null"`
	Count        int      `json:"count" gorm:"not null"`
	Count2       *int     `json:"count2"`
	Covering     string   `json:"covering" gorm:"not null;default:PVC"`
	Comment      string   `json:"comment"`
}

func (InteriorDoor) TableName() string { return "interior_doors" }

type EntranceDoor struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	OrderID     uint    `json:"order_id" gorm:"index;not null"`
	Kind        string  `json:"kind" gorm:"not null"`
	LeafType    string  `json:"leafType" gorm:"not null;default:Single"`
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

func (EntranceDoor) TableName() string { return "entrance_doors" }

type Molding struct {
	ID             uint    `json:"id" gorm:"primaryKey"`
	OrderID        uint    `json:"order_id" gorm:"index;not null"`
	FrameLength    *int    `json:"frameLength"`
	FramePrice     float64 `json:"framePrice" gorm:"not null"`
	FrameCount     float64 `json:"frameCount" gorm:"not null"`
	PlatbandType   string  `json:"platbandType" gorm:"not null"`
	PlatbandFigure *string `json:"platbandFigure"`
	PlatbandLength *int    `json:"platbandLength"`
	PlatbandPrice  float64 `json:"platbandPrice" gorm:"not null"`
	PlatbandCount  float64 `json:"platbandCount" gorm:"not null"`
	RebateBarCount int     `json:"rebateBarCount" gorm:"not null;default:0"`
	Color          string  `json:"color" gorm:"not null"`
	Covering       string  `json:"covering" gorm:"not null;default:Enamel"`
	Comment        string  `json:"comment"`
}

func (Molding) TableName() string { return "moldings" }

type Extension struct {
	ID             uint    `json:"id" gorm:"primaryKey"`
	OrderID        uint    `json:"order_id" gorm:"index;not null"`
	Color          string  `json:"color" gorm:"not null"`
	Covering       string  `json:"covering" gorm:"not null;default:Enamel"`
	Width          int     `json:"width" gorm:"not null"`
	Height         int     `json:"height" gorm:"not null"`
	QuantityPerSet float64 `json:"quantityPerSet" gorm:"not null;default:0.5"`
	TotalArea      float64 `json:"totalArea" gorm:"not null;default:0"`
	Comment        string  `json:"comment"`
	Count          float64 `json:"count" gorm:"not null"`
	Price          float64 `json:"price" gorm:"not null"`
}

func (Extension) TableName() string { return "extensions" }

type Capital struct {
	ID       uint    `json:"id" gorm:"primaryKey"`
	OrderID  uint    `json:"order_id" gorm:"index;not null"`
	Name     string  `json:"name" gorm:"not null"`
	Color    string  `json:"color" gorm:"not null"`
	Covering string  `json:"covering" gorm:"not null;default:Enamel"`
	Width    int     `json:"width" gorm:"not null"`
	Height   int     `json:"height" gorm:"not null"`
	Price    float64 `json:"price" gorm:"not null;default:0"`
	Comment  string  `json:"comment"`
	Count    int     `json:"count" gorm:"not null"`
}

func (Capital) TableName() string { return "capitals" }

type Paneling struct {
	ID             uint    `json:"id" gorm:"primaryKey"`
	OrderID        uint    `json:"order_id" gorm:"index;not null"`
	Color          string  `json:"color" gorm:"not null"`
	Size           string  `json:"size" gorm:"not null"`
	Width          int     `json:"width" gorm:"not null"`
	Height         int     `json:"height" gorm:"not null"`
	Covering       string  `json:"covering" gorm:"not null;default:Enamel"`
	Kind           string  `json:"kind" gorm:"not null;default:smooth"`
	Sizes          Sizes   `json:"sizes" gorm:"type:jsonb"`
	QuantityPerSet float64 `json:"quantityPerSet" gorm:"not null;default:0.5"`
	TotalArea      float64 `json:"totalArea" gorm:"not null;default:0"`
	Count          int     `json:"count" gorm:"not null"`
	Price          float64 `json:"price" gorm:"not null"`
	Comment        string  `json:"comment"`
}

func (Paneling) TableName() string { return "panelings" }

type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type Sizes []Size

func (sizes Sizes) Value() (driver.Value, error) {
	if sizes == nil {
		return []byte("[]"), nil
	}

	return json.Marshal(sizes)
}

func (sizes *Sizes) Scan(value any) error {
	if value == nil {
		*sizes = Sizes{}
		return nil
	}

	switch typedValue := value.(type) {
	case []byte:
		return json.Unmarshal(typedValue, sizes)
	case string:
		return json.Unmarshal([]byte(typedValue), sizes)
	default:
		return fmt.Errorf("unsupported sizes value type %T", value)
	}
}

type Hardware struct {
	ID              uint     `json:"id" gorm:"primaryKey"`
	OrderID         uint     `json:"order_id" gorm:"index;not null"`
	HandleModel     *string  `json:"handleModel"`
	HandleColor     *string  `json:"handleColor"`
	HandleCount     *int     `json:"handleCount"`
	HandlePrice     *float64 `json:"handlePrice"`
	LockCount       *int     `json:"lockCount"`
	LockPrice       *float64 `json:"lockPrice"`
	FixatorCount    *int     `json:"fixatorCount"`
	FixatorPrice    *float64 `json:"fixatorPrice"`
	ClickCount      *int     `json:"clickCount"`
	ClickPrice      *float64 `json:"clickPrice"`
	ThumbturnCount  *int     `json:"thumbturnCount"`
	ThumbturnPrice  *float64 `json:"thumbturnPrice"`
	EscutcheonCount *int     `json:"escutcheonCount"`
	EscutcheonPrice *float64 `json:"escutcheonPrice"`
	CylinderCount   *int     `json:"cylinderCount"`
	CylinderPrice   *float64 `json:"cylinderPrice"`
	BoltCount       *int     `json:"boltCount"`
	BoltPrice       *float64 `json:"boltPrice"`
	HingeCount      *int     `json:"hingeCount"`
	HingePrice      *float64 `json:"hingePrice"`
	DoorStopCount   *int     `json:"doorStopCount"`
	DoorStopPrice   *float64 `json:"doorStopPrice"`
	Comment         string   `json:"comment"`
}

func (Hardware) TableName() string { return "hardwares" }
