package models

type CardType struct {
	ID        uint    `gorm:"primaryKey"`      // 使用 ID 作为主键
	Type      string  `gorm:"unique;not null"` // 唯一标识符
	Price     float64 `gorm:"not null"`        // 价格
	CardCount int     `gorm:"not null"`        // 卡片数量
}

// TableName returns the corresponding database table name for this struct.
func (m CardType) TableName() string {
	return "card_type"
}
