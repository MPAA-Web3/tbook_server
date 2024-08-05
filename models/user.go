package models

import (
	"time"
)

type User struct {
	ID                  uint   `gorm:"primary_key"`
	UserID              string `gorm:"unique;not null"`
	Address             string `json:"address"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Balance             float64 `json:"balance"`
	CardCount           int     `json:"card_count"`
	ProfilePhoto        string  `json:"profile_photo"`                        // 添加头像字段
	JoinedDiscord       bool    `json:"joined_discord" gorm:"default:false"`  // 是否加入 Discord，默认为 false
	JoinedX             bool    `json:"joined_x" gorm:"default:false"`        // 是否加入 X，默认为 false
	JoinedTelegram      bool    `json:"joined_telegram" gorm:"default:false"` // 是否加入 Telegram，默认为 false
	JoinedTelegramGroup bool    `json:"joined_telegram_group" gorm:"default:false"`
}

// TableName returns the corresponding database table name for this struct.
func (m User) TableName() string {
	return "user"
}
