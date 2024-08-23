package models

import (
	"time"
)

// Exhibition represents the exhibition structure in the database
type Exhibition struct {
	ID        uint      `gorm:"primaryKey"`        // 主键
	Name      string    `gorm:"size:255;not null"` // 展览名称
	PlayMode  string    `gorm:"size:255;not null"` // 玩法参数
	Number    string    `gorm:"size:255"`          // 序号
	ImageURL  string    `gorm:"size:255"`          // 展览图片链接
	CreatedAt time.Time `gorm:"autoCreateTime"`    // 创建时间
	UpdatedAt time.Time `gorm:"autoUpdateTime"`    // 更新时间
}

// TableName returns the corresponding database table name for this struct.
func (m Exhibition) TableName() string {
	return "exhibition"
}
