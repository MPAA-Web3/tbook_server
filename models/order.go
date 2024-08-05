package models

import "time"

type Order struct {
	ID              uint      `gorm:"primaryKey"`
	UserID          string    `json:"user_id"`
	Address         string    `json:"address"`
	Status          string    `json:"status"`
	Amount          float64   `json:"amount"`
	TransactionHash string    `json:"transaction_hash"` // 新增的交易哈希字段
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TableName returns the corresponding database table name for this struct.
func (m Order) TableName() string {
	return "order"
}
