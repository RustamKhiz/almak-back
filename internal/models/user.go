package models

import "time"

type User struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Login     string    `json:"login" gorm:"uniqueIndex;not null"`
	Password  string    `json:"-" gorm:"not null"`
	Name      string    `json:"name"`
	Role      UserRole  `json:"role" gorm:"not null;default:manager;check:user_role_valid,role IN ('admin','manager')"`
	CreatedAt time.Time `json:"created_at"`
}

type UserRole string

const (
	RoleAdmin   UserRole = "admin"
	RoleManager UserRole = "manager"
)

func (r UserRole) Valid() bool { return r == RoleAdmin || r == RoleManager }

type Permission string

const (
	OrdersRead     Permission = "orders.read"
	OrdersWrite    Permission = "orders.write"
	OrdersDelete   Permission = "orders.delete"
	PaymentsWrite  Permission = "payments.write"
	DiscountsWrite Permission = "discounts.write"
	CatalogsRead   Permission = "catalogs.read"
	CatalogsWrite  Permission = "catalogs.write"
	AnalyticsRead  Permission = "analytics.read"
	DocumentsRead  Permission = "documents.read"
	DraftsWrite    Permission = "drafts.write"
	AssistantUse   Permission = "assistant.use"
)

// Both roles intentionally have identical access in R1. Future restrictions
// belong here; the server and /me must use the same policy.
func Permissions(role UserRole) []Permission {
	if !role.Valid() {
		return []Permission{}
	}
	return []Permission{OrdersRead, OrdersWrite, OrdersDelete, PaymentsWrite, DiscountsWrite, CatalogsRead, CatalogsWrite, AnalyticsRead, DocumentsRead, DraftsWrite, AssistantUse}
}

func (u User) Can(permission Permission) bool {
	for _, granted := range Permissions(u.Role) {
		if granted == permission {
			return true
		}
	}
	return false
}

type UserProfile struct {
	ID          uint         `json:"id"`
	Login       string       `json:"login"`
	Name        string       `json:"name"`
	Role        UserRole     `json:"role"`
	Permissions []Permission `json:"permissions"`
}

func (u User) Profile() UserProfile {
	return UserProfile{ID: u.ID, Login: u.Login, Name: u.Name, Role: u.Role, Permissions: Permissions(u.Role)}
}
