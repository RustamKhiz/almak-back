package models

import "time"

type Catalog struct {
	ID        uint          `json:"id" gorm:"primaryKey"`
	Name      string        `json:"name" gorm:"not null;uniqueIndex"`
	Items     []CatalogItem `json:"items,omitempty" gorm:"foreignKey:CatalogID;constraint:OnDelete:CASCADE"`
	CreatedAt time.Time     `json:"created_at"`
}

type CatalogItem struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CatalogID uint      `json:"catalogId" gorm:"not null;index"`
	Value     string    `json:"value" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
}
