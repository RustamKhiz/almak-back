package models

import (
	"almak-back/internal/draftorders"
	"time"
)

// Messages are separate from the CAS document: saving a form cannot replace history.
type OrderDraft struct {
	SchemaVersion int                  `json:"schemaVersion" gorm:"not null;default:1"`
	ID            string               `json:"id" gorm:"primaryKey;size:36"`
	Kind          string               `json:"kind" gorm:"not null"`
	Revision      int                  `json:"revision" gorm:"not null"`
	State         string               `json:"state" gorm:"not null;index"`
	Document      draftorders.Document `json:"document" gorm:"serializer:json;type:jsonb;not null"`
	CreatedBy     *uint                `json:"createdBy" gorm:"index"`
	UpdatedBy     uint                 `json:"updatedBy"`
	ImportedBy    *uint                `json:"importedBy,omitempty"`
	ImportKey     *string              `json:"-" gorm:"uniqueIndex"`
	ImportHash    string               `json:"-"`
	CreateRequest string               `json:"-"`
	OrderID       *uint                `json:"orderId,omitempty"`
	Creator       *User                `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	Editor        *User                `json:"editor,omitempty" gorm:"foreignKey:UpdatedBy"`
	CreatedAt     time.Time            `json:"createdAt"`
	UpdatedAt     time.Time            `json:"updatedAt"`
	Messages      []DraftMessage       `json:"messages" gorm:"foreignKey:DraftID"`
}

type DraftMessage struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	DraftID   string    `json:"-" gorm:"size:36;not null;index"`
	Role      string    `json:"role" gorm:"not null"`
	Text      string    `json:"text" gorm:"not null"`
	AuthorID  *uint     `json:"authorId"`
	RequestID string    `json:"-" gorm:"index"`
	CreatedAt time.Time `json:"createdAt"`
}

type DraftAIRequest struct {
	CanonicalID  string `gorm:"not null;default:''"`
	ID           string `gorm:"primaryKey;size:36"`
	DraftID      string `gorm:"size:36;not null;index"`
	UserID       uint
	Text         string
	BaseRevision int
	Status       string
	UpdatedAt    time.Time
	CreatedAt    time.Time
}

type DraftMutation struct {
	ID        string `gorm:"primaryKey;size:36"`
	DraftID   string `gorm:"size:36;not null;index"`
	Hash      string
	Revision  int
	CreatedAt time.Time
}
