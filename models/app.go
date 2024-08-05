package models

type APP struct {
	ID                     uint  `gorm:"primaryKey"`         // 使用 ID 作为主键
	DailyCardPurchaseLimit int   `gorm:"not null"`           // 每日卡片购买次数限制
	TelegramChannelAmount  int64 `gorm:"not null;default:0"` // Telegram 频道奖励发放额度
	TelegramGroupAmount    int64 `gorm:"not null;default:0"` // Telegram 群组奖励发放额度
	TwitterAmount          int64 `gorm:"not null;default:0"` // Twitter 奖励发放额度
	DiscordAmount          int64 `gorm:"not null;default:0"` // Discord 奖励发放额度
}

// TableName returns the corresponding database table name for this struct.
func (m APP) TableName() string {
	return "app"
}
