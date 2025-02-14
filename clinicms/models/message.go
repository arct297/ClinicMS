package models

import (
	"gorm.io/gorm"
	"time"
)

type Message struct {
	ID          uint           `gorm:"primaryKey"`
	ChatID      uint           `gorm:"not null;index" json:"chat_id"`
	SenderID    uint           `gorm:"not null" json:"sender_id"`
	SenderRole  string         `gorm:"not null" json:"sender_role"`
	MessageText string         `gorm:"not null" json:"message_text"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}
