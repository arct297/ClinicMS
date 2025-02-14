package models

import (
	"time"
)

type ChatRead struct {
	ID            uint      `gorm:"primaryKey"`
	ChatID        uint      `gorm:"not null;index" json:"chat_id"`
	UserID        uint      `gorm:"not null" json:"user_id"`
	UserRole      string    `gorm:"not null" json:"user_role"`
	LastReadMsgID *uint     `gorm:"column:last_read_message_id" json:"last_read_message_id,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}
