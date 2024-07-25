package models

import (
	"time"
)

type User struct {
	ID           uint   `gorm:"primary_key"`
	UserID       string `gorm:"unique;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Balance      float64 `json:"balance"`
	CardCount    int     `json:"card_count"`
	ProfilePhoto string  `json:"profile_photo"` // 添加头像字段
}

// TableName returns the corresponding database table name for this struct.
func (m User) TableName() string {
	return "user"
}
