package models

import "time"

type Invitation struct {
	ID             uint      `gorm:"primary_key"`
	InviterID      string    `gorm:"not null"` // 邀请者用户ID
	InviterAddress string    `gorm:"not null"`
	InviteeUserID  string    `gorm:"not null"` // 被邀请者用户ID
	InviteeAddress string    `gorm:"not null"`
	Level          int       `gorm:"not null"` // 邀请级别 (1: 一级邀请, 2: 二级邀请)
	CreatedAt      time.Time `gorm:"not null"` // 邀请记录创建时间
}

// TableName returns the corresponding database table name for this struct.
func (m Invitation) TableName() string {
	return "invitation"
}
