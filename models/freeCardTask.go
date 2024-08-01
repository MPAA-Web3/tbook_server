package models

import "time"

type FreeCardTask struct {
	ID        uint       `gorm:"primary_key"`
	UserID    string     `gorm:"not null"`
	CreatedAt time.Time  `gorm:"not null"`
	GrantedAt *time.Time `gorm:"default:null"` // 发放时间，默认空值
	IsGranted bool       `gorm:"not null"`     // 是否发放
}

// TableName returns the corresponding database table name for this struct.
func (m FreeCardTask) TableName() string {
	return "free_card_task"
}
