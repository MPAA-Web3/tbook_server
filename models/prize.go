package models

import "time"

// Prize represents the prize structure in the database
type Prize struct {
	ID                uint      `gorm:"primaryKey"`        // 主键
	Name              string    `gorm:"size:255;not null"` // 奖品名称
	Type              string    `gorm:"size:255;not null"` // 奖品类型  积分，卡,实物
	Value             string    `gorm:"size:255;not null"` // 奖品值，例如积分数
	Probability       float64   `gorm:"not null"`          // 奖品概率
	IsTimeBased       bool      `gorm:"not null"`          // 是否基于时间
	StartTime         time.Time // 开始时间
	EndTime           time.Time // 结束时间
	ImageURL          string    `gorm:"size:255"`          // 奖品图片链接
	PlayMode          string    `gorm:"size:255;not null"` // 玩法参数
	IsAutoDistributed bool      `gorm:"not null"`          // 是否自动发放
	Quota             int64     `gorm:"default:0"`         // 抽中名额限制，0表示无限制
	DistributedCount  int64     `gorm:"default:0"`         // 已发放数量
	CreatedAt         time.Time `gorm:"autoCreateTime"`    // 创建时间
	UpdatedAt         time.Time `gorm:"autoUpdateTime"`    // 更新时间
}

// TableName returns the corresponding database table name for this struct.
func (m Prize) TableName() string {
	return "prize"
}
