package models

import (
	"time"
)

// AchievementReward 记录成就奖励的发放情况
type AchievementReward struct {
	ID              uint      `gorm:"primaryKey"`
	UserID          string    `gorm:"not null"` // 用户ID
	AchievementName string    `gorm:"not null"` // 成就名称 10Friend
	RewardType      string    `gorm:"not null"` // 奖励类型，例如"Balance",
	Amount          int64     `gorm:"not null"` // 奖励数量
	CreatedAt       time.Time // 奖励发放时间
}

// TableName returns the corresponding database table name for this struct.
func (m AchievementReward) TableName() string {
	return "achievement_reward"
}
