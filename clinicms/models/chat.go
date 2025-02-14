package models

import (
	"gorm.io/gorm"
	"time"
)

type Chat struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	ClientID  uint           `gorm:"not null;index" json:"client_id"`
	AdminID   *uint          `json:"admin_id,omitempty"`
	Status    string         `gorm:"not null;default:'active'" json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}
