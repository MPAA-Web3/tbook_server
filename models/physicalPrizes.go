package models

import "time"

// PhysicalPrize represents the structure of a physical prize in the database.
type PhysicalPrize struct {
	ID        uint      `gorm:"primaryKey"`        // 主键
	UserID    uint      `gorm:"not null"`          // 用户ID
	PrizeName string    `gorm:"size:255;not null"` // 奖品名称
	WinTime   time.Time `gorm:"not null"`          // 中奖时间
	CreatedAt time.Time `gorm:"autoCreateTime"`    // 创建时间
	UpdatedAt time.Time `gorm:"autoUpdateTime"`    // 更新时间
}

// TableName returns the corresponding database table name for this struct.
func (PhysicalPrize) TableName() string {
	return "physical_prizes"
}
