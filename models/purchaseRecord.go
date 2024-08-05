package models

import (
	"time"
)

// PurchaseRecord 表示购买记录的模型
type PurchaseRecord struct {
	ID           uint      `gorm:"primaryKey"` // 主键 ID
	UserID       string    `gorm:"not null"`   // 用户 ID
	PurchaseTime time.Time `gorm:"not null"`   // 购买时间
	PointsSpent  float64   `gorm:"not null"`   // 花费积分
	CardCount    int       `gorm:"not null"`   // 获得卡片数量
	Type         string    `gorm:"not null"`   // 记录类型
}

// TableName returns the corresponding database table name for this struct.
func (m PurchaseRecord) TableName() string {
	return "purchase_record"
}
