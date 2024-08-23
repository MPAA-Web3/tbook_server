package handle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"tbooks/configs"
	"tbooks/daos"
	"tbooks/errorss"
	"tbooks/models"
	"time"
)

type Prize struct {
	Name              string    `json:"name"`
	Value             string    `json:"value"`
	ImageURL          string    `json:"image_url"`           // 图片 URL 字段
	Probability       float64   `json:"probability"`         // 抽奖概率字段
	IsTimeBased       bool      `json:"is_time_based"`       // 是否基于时间的抽奖
	StartTime         time.Time `json:"start_time"`          // 抽奖开始时间
	EndTime           time.Time `json:"end_time"`            // 抽奖结束时间
	IsAutoDistributed bool      `json:"is_auto_distributed"` // 是否自动发放字段
}

//var prizes = map[string]map[string]Prize{
//	"1": { // 玩法 1: 抽奖四个奖品
//		"1": {
//			Name:              "100points",
//			Value:             "100",
//			ImageURL:          "1",
//			Probability:       0.25,        // 25% 概率
//			IsTimeBased:       false,       // 不基于时间的抽奖
//			StartTime:         time.Time{}, // 未设置时间
//			EndTime:           time.Time{}, // 未设置时间
//			IsAutoDistributed: true,        // 自动发放
//		},
//		"2": {
//			Name:              "200points",
//			Value:             "200",
//			ImageURL:          "2",
//			Probability:       0.25,
//			IsTimeBased:       false,
//			StartTime:         time.Time{},
//			EndTime:           time.Time{},
//			IsAutoDistributed: false, // 手动发放
//		},
//		"3": {
//			Name:              "300points",
//			Value:             "300",
//			ImageURL:          "3",
//			Probability:       0.25,
//			IsTimeBased:       false,
//			StartTime:         time.Time{},
//			EndTime:           time.Time{},
//			IsAutoDistributed: false, // 手动发放
//		},
//		"4": {
//			Name:              "1card",
//			Value:             "1",
//			ImageURL:          "4",
//			Probability:       0.25,
//			IsTimeBased:       true,                            // 开启时间抽奖
//			StartTime:         time.Now().Add(-time.Hour * 24), // 开始时间：一天前
//			EndTime:           time.Now().Add(time.Hour * 24),  // 结束时间：一天后
//			IsAutoDistributed: true,                            // 自动发放
//		},
//		"5": {
//			Name:              "200points",
//			Value:             "200",
//			ImageURL:          "2",
//			Probability:       0.25,
//			IsTimeBased:       false,
//			StartTime:         time.Time{},
//			EndTime:           time.Time{},
//			IsAutoDistributed: false, // 手动发放
//		},
//	},
//
//	//"2": { // 玩法 2: 大转盘八个奖品
//	//	"1": {Name: "50points", Value: "50", ImageURL: "1"},
//	//	"2": {Name: "100points", Value: "100", ImageURL: "2"},
//	//	"3": {Name: "150points", Value: "150", ImageURL: "3"},
//	//	"4": {Name: "200points", Value: "200", ImageURL: "4"},
//	//	"5": {Name: "250points", Value: "250", ImageURL: "5"},
//	//	"6": {Name: "300points", Value: "300", ImageURL: "6"},
//	//	"7": {Name: "1card", Value: "1", ImageURL: "7"},
//	//	"8": {Name: "400points", Value: "400", ImageURL: "8"},
//	//},
//}

func LuckDraw(c *gin.Context) {
	var input struct {
		UserID   string `json:"userid" binding:"required"`
		PlayMode string `json:"playmode" binding:"required"` // 添加玩法参数
	}

	// 绑定 JSON 输入到结构体
	if err := c.ShouldBindJSON(&input); err != nil {
		errorss.HandleError(c, 400, err)
		return
	}

	// 获取用户的卡片次数
	cardCount, err := configs.Rdb.Get(c, input.UserID+"_card_count").Int()
	if err == redis.Nil {
		errorss.HandleError(c, 404, errors.New("User not found")) // 用户未找到
		return
	} else if err != nil {
		errorss.HandleError(c, 500, err) // 获取用户卡片次数失败
		return
	}

	// 检查卡片次数
	if cardCount <= 0 {
		errorss.HandleError(c, 403, errors.New("Insufficient card")) // 卡片次数不足
		return
	}

	// 扣除一次卡片次数
	if err := configs.Rdb.Decr(c, input.UserID+"_card_count").Err(); err != nil {
		errorss.HandleError(c, 500, err) // 更新卡片次数失败
		return
	}

	// 选择对应玩法的奖品
	var availablePrizes []models.Prize
	result := daos.DB.Where("play_mode = ?", input.PlayMode).Find(&availablePrizes)
	if result.Error != nil {
		errorss.HandleError(c, 500, result.Error) // 查询奖品失败
		return
	}

	// 获取当前时间
	now := time.Now()

	// 筛选有效的奖品（时间范围检查和名额限制）
	validPrizes := make([]models.Prize, 0)
	for _, prize := range availablePrizes {
		if (!prize.IsTimeBased || (now.After(prize.StartTime) && now.Before(prize.EndTime))) &&
			(prize.Quota == 0 || prize.DistributedCount < prize.Quota) {
			validPrizes = append(validPrizes, prize)
		}
	}

	// 如果没有有效的奖品
	if len(validPrizes) == 0 {
		errorss.HandleError(c, 500, errors.New("No valid prizes available at this time"))
		return
	}

	// 计算总概率
	totalProbability := 0.0
	for _, prize := range validPrizes {
		totalProbability += prize.Probability
	}

	// 生成随机数
	rand.Seed(time.Now().UnixNano())
	random := rand.Float64() * totalProbability

	// 根据随机数选择奖品（仅考虑有效奖品）
	var selectedPrize models.Prize
	cumulativeProbability := 0.0
	for _, prize := range validPrizes {
		cumulativeProbability += prize.Probability
		if random < cumulativeProbability {
			selectedPrize = prize
			break
		}
	}

	// 更新发放数量
	if selectedPrize.Quota > 0 {
		// 如果奖品已达到名额限制，则不允许发放
		if err := daos.DB.Model(&models.Prize{}).Where("id = ?", selectedPrize.ID).
			Updates(map[string]interface{}{
				"distributed_count": gorm.Expr("distributed_count + ?", 1),
			}).Error; err != nil {
			errorss.HandleError(c, 500, err) // 更新发放数量失败
			return
		}

		// 检查是否超过名额限制
		var currentCount int64
		if err := daos.DB.Model(&models.Prize{}).Where("id = ?", selectedPrize.ID).
			Pluck("distributed_count", &currentCount).Error; err != nil {
			errorss.HandleError(c, 500, err) // 获取发放数量失败
			return
		}
		if currentCount > selectedPrize.Quota {
			// 奖品发放数量已达上限，重新抽奖
			// 将当前奖品从有效奖品列表中移除
			var updatedPrizes []models.Prize
			for _, prize := range validPrizes {
				if prize.ID != selectedPrize.ID {
					updatedPrizes = append(updatedPrizes, prize)
				}
			}
			validPrizes = updatedPrizes

			// 如果有效奖品列表为空，返回错误
			if len(validPrizes) == 0 {
				errorss.HandleError(c, 500, errors.New("No valid prizes available after updating"))
				return
			}

			// 重新计算总概率
			totalProbability = 0.0
			for _, prize := range validPrizes {
				totalProbability += prize.Probability
			}

			// 重新生成随机数
			random = rand.Float64() * totalProbability

			// 根据随机数重新选择奖品
			cumulativeProbability = 0.0
			for _, prize := range validPrizes {
				cumulativeProbability += prize.Probability
				if random < cumulativeProbability {
					selectedPrize = prize
					break
				}
			}
		}
	}

	// 处理奖品
	switch selectedPrize.Type {
	case "points":
		// 奖品是积分
		balance, err := strconv.ParseFloat(selectedPrize.Value, 64)
		if err != nil {
			errorss.HandleError(c, 500, err) // 解析余额失败
			return
		}
		userBalanceKey := input.UserID + "_balance"
		if err := configs.Rdb.IncrByFloat(c, userBalanceKey, balance).Err(); err != nil {
			errorss.HandleError(c, 500, err) // 更新余额失败
			return
		}
		newBalance, _ := configs.Rdb.Get(c, userBalanceKey).Float64()
		errorss.JsonSuccess(c, gin.H{
			"message":    "Congratulations! You have won the prize!",
			"name":       selectedPrize.Value + selectedPrize.Name,
			"prize":      selectedPrize.Value,
			"balance":    newBalance,
			"number":     selectedPrize.ImageURL, // 包含奖品图片链接
			"card_count": cardCount - 1,
		})

	case "card":
		// 奖品是抽奖卡
		if err := configs.Rdb.Incr(c, input.UserID+"_card_count").Err(); err != nil {
			errorss.HandleError(c, 500, err) // 更新卡片次数失败
			return
		}
		newCardCount, _ := configs.Rdb.Get(c, input.UserID+"_card_count").Int()
		errorss.JsonSuccess(c, gin.H{
			"message":    "Congratulations! You have won a lottery card!",
			"name":       selectedPrize.Name,
			"prize":      selectedPrize.Value,
			"card_count": newCardCount,
			"number":     selectedPrize.ImageURL, // 包含奖品图片链接
		})

	case "material":
		// 奖品是物料
		if err := daos.DB.Model(&models.Prize{}).Where("id = ?", selectedPrize.ID).
			Updates(map[string]interface{}{
				"distributed_count": gorm.Expr("distributed_count + ?", 1),
			}).Error; err != nil {
			errorss.HandleError(c, 500, err) // 更新发放数量失败
			return
		}

		// 创建实物中奖记录
		prizeRecord := models.PhysicalPrize{
			UserID:    1, // 假设用户ID是1，实际中应该从请求中获取
			PrizeName: selectedPrize.Name,
			WinTime:   time.Now(),
		}

		// 插入实物中奖记录到数据库
		if err := daos.DB.Create(&prizeRecord).Error; err != nil {
			errorss.HandleError(c, 500, err) // 插入实物中奖记录失败
			return
		}
		errorss.JsonSuccess(c, gin.H{
			"message":    "Congratulations! You have won a material prize!",
			"name":       selectedPrize.Name,
			"prize":      selectedPrize.Value,
			"card_count": cardCount - 1,
			"number":     selectedPrize.ImageURL, // 包含奖品图片链接
		})

	case "thank you":
		{
			errorss.JsonSuccess(c, gin.H{
				"message":    "Thank you for participating!",
				"name":       selectedPrize.Name,
				"prize":      selectedPrize.Value,
				"card_count": cardCount - 1,
				"number":     selectedPrize.ImageURL, // 包含奖品图片链接
			})
		}

	default:
		errorss.HandleError(c, 500, errors.New("Unknown prize type")) // 未知奖品类型
	}
}

//func LuckDraw(c *gin.Context) {
//	var input struct {
//		UserID   string `json:"userid" binding:"required"`
//		PlayMode string `json:"playmode" binding:"required"` // 添加玩法参数
//	}
//
//	// 绑定 JSON 输入到结构体
//	if err := c.ShouldBindJSON(&input); err != nil {
//		errorss.HandleError(c, 400, err)
//		return
//	}
//	// 获取用户的卡片次数
//	cardCount, err := configs.Rdb.Get(c, input.UserID+"_card_count").Int()
//	if err == redis.Nil {
//		errorss.HandleError(c, 404, errors.New("User not found")) // 用户未找到
//		return
//	} else if err != nil {
//		errorss.HandleError(c, 500, err) // 获取用户卡片次数失败
//		return
//	}
//	// 检查卡片次数
//	if cardCount <= 0 {
//		errorss.HandleError(c, 403, errors.New("Insufficient card ")) // 卡片次数不足
//		return
//	}
//	// 扣除一次卡片次数
//	if err := configs.Rdb.Decr(c, input.UserID+"_card_count").Err(); err != nil {
//		errorss.HandleError(c, 500, err) // 更新卡片次数失败
//		return
//	}
//	// 选择对应玩法的奖品
//	availablePrizes, ok := prizes[input.PlayMode]
//	if !ok {
//		errorss.HandleError(c, 400, errors.New("Invalid PlayMode parameter")) // 无效的玩法参数
//		return
//	}
//	// 随机选择一个奖品
//	rand.Seed(time.Now().UnixNano())
//	prizeKey := []string{"1", "2", "3", "4", "5", "6", "7", "8"}[rand.Intn(len(availablePrizes))]
//	prize := availablePrizes[prizeKey]
//
//	// 处理奖品
//	switch prize.Name {
//	case "100points", "200points", "300points", "400points", "50points", "150points", "250points":
//		// 奖品是余额
//		balance, err := strconv.ParseFloat(prize.Value, 64)
//		if err != nil {
//			errorss.HandleError(c, 500, err) // 解析余额失败
//			return
//		}
//		userBalanceKey := input.UserID + "_balance"
//		if err := configs.Rdb.IncrByFloat(c, userBalanceKey, balance).Err(); err != nil {
//			errorss.HandleError(c, 500, err) // 更新余额失败
//			return
//		}
//		newBalance, _ := configs.Rdb.Get(c, userBalanceKey).Float64()
//		errorss.JsonSuccess(c, gin.H{
//			"message":    "congratulations! You have won the prize!",
//			"prize":      prize.Name,
//			"balance":    newBalance,
//			"number":     prize.ImageURL, // 包含奖品图片链接
//			"card_count": cardCount - 1,
//		})
//
//	case "1card":
//		// 奖品是抽奖卡
//		if err := configs.Rdb.Incr(c, input.UserID+"_card_count").Err(); err != nil {
//			errorss.HandleError(c, 500, err) // 更新卡片次数失败
//			return
//		}
//		newCardCount, _ := configs.Rdb.Get(c, input.UserID+"_card_count").Int()
//		errorss.JsonSuccess(c, gin.H{
//			"message":    "congratulations! You have won a lottery card!",
//			"prize":      prize.Name,
//			"card_count": newCardCount,
//			"number":     prize.ImageURL, // 包含奖品图片链接
//		})
//
//	default:
//		errorss.HandleError(c, 500, errors.New("Unknown prize type")) // 未知奖品类型
//	}
//}

func UserBalance(c *gin.Context) {
	var input struct {
		UserID string `json:"userid" binding:"required"`
	}

	// 绑定 JSON 输入到结构体
	if err := c.ShouldBindJSON(&input); err != nil {
		errorss.HandleError(c, 400, err) // 无效的 JSON 输入
		return
	}

	// 获取 Redis 上的键
	cardCountKey := input.UserID + "_card_count"
	balanceKey := input.UserID + "_balance"

	// 从 Redis 获取卡片数量和余额
	redisCardCountStr, err := configs.Rdb.Get(context.Background(), cardCountKey).Result()
	if err == redis.Nil {
		// Redis 中没有卡片数量，尝试从数据库获取并缓存
		var user models.User
		if err := daos.DB.Where("user_id = ?", input.UserID).First(&user).Error; err != nil {
			errorss.HandleError(c, 404, err) // 用户未找到
			return
		}
		// 更新 Redis
		err = configs.Rdb.Set(context.Background(), cardCountKey, user.CardCount, 0).Err()
		if err != nil {
			log.Println("Failed to cache user card count to Redis:", err)
		}
		redisCardCountStr = strconv.Itoa(user.CardCount)
	} else if err != nil {
		errorss.HandleError(c, 500, err) // 无法获取卡片数量
		return
	}

	redisBalanceStr, err := configs.Rdb.Get(context.Background(), balanceKey).Result()
	if err == redis.Nil {
		// Redis 中没有余额，尝试从数据库获取并缓存
		var user models.User
		if err := daos.DB.Where("user_id = ?", input.UserID).First(&user).Error; err != nil {
			errorss.HandleError(c, 404, err) // 用户未找到
			return
		}
		// 更新 Redis
		err = configs.Rdb.Set(context.Background(), balanceKey, user.Balance, 0).Err()
		if err != nil {
			log.Println("Failed to cache user balance to Redis:", err)
		}
		redisBalanceStr = strconv.FormatFloat(user.Balance, 'f', -1, 64)
	} else if err != nil {
		errorss.HandleError(c, 500, err) // 无法获取余额
		return
	}

	// 从 Redis 中获取一级邀请数量
	var friendsCount int64
	if err := daos.DB.Model(&models.Invitation{}).Where("inviter_id = ? AND level = ?", input.UserID, 1).Count(&friendsCount).Error; err != nil {
		errorss.HandleError(c, 500, err) // 无法获取邀请数量
		return
	}

	// 返回用户的余额和卡片次数
	errorss.JsonSuccess(c, gin.H{
		"user_id":       input.UserID,
		"balance":       redisBalanceStr,
		"card_count":    redisCardCountStr,
		"friends_count": friendsCount,
	})
}
func BuyCard(c *gin.Context) {
	// 定义结构体以绑定请求中的 JSON 数据
	var input struct {
		UserID string `json:"userid" binding:"required"`
		Type   string `json:"type" binding:"required"`
	}

	// 将 JSON 请求体绑定到 input 结构体
	if err := c.ShouldBindJSON(&input); err != nil {
		errorss.HandleError(c, http.StatusBadRequest, err) // 无效的 JSON 格式
		return
	}

	// 从数据库中检索用户信息
	var user models.User
	if err := daos.DB.Where("user_id = ?", input.UserID).First(&user).Error; err != nil {
		errorss.HandleError(c, http.StatusNotFound, err) // 用户未找到
		return
	}

	// 获取每日购买限制
	var appConfig models.APP
	if err := daos.DB.First(&appConfig).Error; err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	// 检查用户当天的购买次数是否达到限制
	reachedLimit, err := hasReachedPurchaseLimit(input.UserID, appConfig.DailyCardPurchaseLimit, input.Type)
	if err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err) // 查询购买记录失败
		return
	}
	if reachedLimit {
		errorss.HandleError(c, http.StatusForbidden, errors.New("Exceeding the number of times")) // 购买次数超限
		return
	}

	// 从数据库中检索卡片类型信息
	var cardType models.CardType
	if err := daos.DB.Where("type = ?", input.Type).First(&cardType).Error; err != nil {
		errorss.HandleError(c, http.StatusNotFound, err) // 卡片类型未找到
		return
	}

	// 检查用户是否有足够的余额购买卡片
	if user.Balance < cardType.Price {
		errorss.HandleError(c, http.StatusPaymentRequired, nil) // 余额不足
		return
	}

	// 扣除余额并增加卡片数量
	user.Balance -= cardType.Price
	user.CardCount += cardType.CardCount
	user.UpdatedAt = time.Now() // 更新最后修改时间

	// 保存更新后的用户信息到数据库
	if err := daos.DB.Save(&user).Error; err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err) // 更新用户失败
		return
	}

	// 记录购买操作
	purchaseRecord := models.PurchaseRecord{
		UserID:       input.UserID,
		PurchaseTime: time.Now(),
		PointsSpent:  cardType.Price,
		CardCount:    cardType.CardCount,
		Type:         cardType.Type,
	}
	if err := daos.DB.Create(&purchaseRecord).Error; err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err) // 记录购买失败
		return
	}

	// 更新 Redis 缓存中的用户数据
	userJSON, err := json.Marshal(user)
	if err != nil {
		log.Println("Failed to marshal user data:", err)
	} else {
		err = configs.Rdb.Set(configs.Ctx, "user:"+user.UserID, userJSON, 0).Err()
		if err != nil {
			log.Println("Failed to cache user data to Redis:", err)
		}
	}

	// 缓存用户的卡片数量
	cardCountKey := user.UserID + "_card_count"
	err = configs.Rdb.Set(configs.Ctx, cardCountKey, user.CardCount, 0).Err()
	if err != nil {
		log.Println("Failed to cache user card count to Redis:", err)
	}

	// 缓存用户的余额
	balanceKey := user.UserID + "_balance"
	err = configs.Rdb.Set(configs.Ctx, balanceKey, user.Balance, 0).Err()
	if err != nil {
		log.Println("Failed to cache user balance to Redis:", err)
	}

	// 返回成功消息和更新后的用户数据
	errorss.JsonSuccess(c, gin.H{
		"message": "Card purchased successfully",
		"user":    user,
	})
}

// Check if the user has reached the purchase limit for today
// Check if the user has reached the purchase limit for today based on a specific type
func hasReachedPurchaseLimit(userID string, limit int, purchaseType string) (bool, error) {
	// 获取当前日期
	startOfDay := time.Now().Truncate(24 * time.Hour)
	endOfDay := startOfDay.Add(24 * time.Hour)

	var count int64
	// 查询当天用户指定类型的购买记录数
	err := daos.DB.Model(&models.PurchaseRecord{}).
		Where("user_id = ? AND purchase_time >= ? AND purchase_time < ? AND type = ?", userID, startOfDay, endOfDay, purchaseType).
		Count(&count).Error

	if err != nil {
		return false, err // 查询失败
	}

	// 检查是否超过限制
	return count >= int64(limit), nil
}

func CreateUser(c *gin.Context) {
	var input struct {
		UserID            string `json:"userid" binding:"required"`
		Address           string `json:"address"`
		InvitationAddress string `json:"invitation_address"`
	}

	// Bind JSON input to the struct
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if the user already exists
	var existingUser models.User
	if err := daos.DB.Where("user_id = ?", input.UserID).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	// Create the user with default balance and card count
	user := models.User{
		UserID:    input.UserID,
		Balance:   0,     // Default balance
		CardCount: 10000, // Default card count
		Address:   input.Address,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := daos.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Handle invitation if provided
	if input.InvitationAddress != "" {
		// Find the inviter based on the provided invitation address
		var inviter models.User
		if err := daos.DB.Where("user_id = ?", input.UserID).First(&inviter).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invitation address not found"})
			return
		}

		// Check if the inviter has an upper-level inviter
		var upperInviter models.Invitation
		if err := daos.DB.Where("invitee_userid = ? AND level = ?", inviter.UserID, 1).First(&upperInviter).Error; err == nil {
			// Inviter has an upper-level inviter, so this user becomes a level 2 invitee
			invitation := models.Invitation{
				InviterID:      upperInviter.InviterID,
				InviterAddress: upperInviter.InviterAddress,
				InviteeUserID:  user.UserID,
				InviteeAddress: user.Address,
				Level:          2, // This is a level 2 invitation
				CreatedAt:      time.Now(),
			}

			if err := daos.DB.Create(&invitation).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else {
			// No upper-level inviter, so this user is a level 1 invitee
			invitation := models.Invitation{
				InviterID:      inviter.UserID,
				InviterAddress: inviter.Address,
				InviteeUserID:  user.UserID,
				InviteeAddress: user.Address,
				Level:          1, // This is a level 1 invitation
				CreatedAt:      time.Now(),
			}

			if err := daos.DB.Create(&invitation).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	}
	// Respond with the created user
	c.JSON(http.StatusOK, gin.H{"message": "User created successfully", "user": user})
}

type Leaderboard struct {
	Rank         int     `json:"rank"`
	UserID       string  `json:"user_id"`
	Balance      float64 `json:"balance"`
	ProfilePhoto string  `json:"profile_photo"`
	Address      string  `json:"address"`
}

func GetLeaderboard(c *gin.Context) {
	var leaderboard []Leaderboard

	// 查询前100位用户按照 balance 降序排序，且地址不为空
	result := daos.DB.Model(&models.User{}).
		Select("user_id, balance, profile_photo, address").
		Where("address != ?", "").
		Order("balance DESC").
		Limit(100).
		Find(&leaderboard)
	if result.Error != nil {
		errorss.HandleError(c, http.StatusInternalServerError, result.Error) // 数据库查询错误
		return
	}

	// 设置排行榜的排名
	for i := range leaderboard {
		leaderboard[i].Rank = i + 1
	}

	errorss.JsonSuccess(c, leaderboard)
}

type RegularTask struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	Completed   bool   `json:"completed"`
}

// IncrementBalance 增加用户余额
func IncrementBalance(userID string, amount int64) error {
	ctx := context.Background()
	balanceKey := userID + "_balance"
	_, err := configs.Rdb.IncrBy(ctx, balanceKey, amount).Result()
	return err
}

func GetRegularTasks(c *gin.Context) {
	userID := c.Query("user_id")

	// 在数据库中查找用户
	var user models.User
	if err := daos.DB.Where("user_id = ?", userID).First(&user).Error; err != nil {
		errorss.HandleError(c, http.StatusNotFound, err) // 用户未找到
		return
	}

	// 查询用户邀请一级数量
	var inviteCount int64
	if err := daos.DB.Model(&models.Invitation{}).Where("inviter_id = ? AND level = ?", userID, 1).Count(&inviteCount).Error; err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err) // 无法查询邀请数量
		return
	}

	// 检查是否已完成“Invite 10 Frens”任务，并奖励用户
	if inviteCount >= 10 {
		var existingReward models.AchievementReward
		if err := daos.DB.Where("user_id = ? AND achievement_name = ?", userID, "10Friend").First(&existingReward).Error; err != nil {
			// 如果没有记录，则发放奖励
			if err := IncrementBalance(userID, 1000); err != nil {
				errorss.HandleError(c, http.StatusInternalServerError, err) // Redis 更新失败
				return
			}

			// 创建奖励记录
			reward := models.AchievementReward{
				UserID:          userID,
				AchievementName: "10Friend",
				RewardType:      "Balance",
				Amount:          1000,
				CreatedAt:       time.Now(),
			}

			if err := daos.DB.Create(&reward).Error; err != nil {
				errorss.HandleError(c, http.StatusInternalServerError, err) // 数据库插入失败
				return
			}
		}
	}

	// 定义任务列表，完成状态写死
	tasks := []RegularTask{
		{Name: "Invite 10 Frens", Description: "邀请10个朋友", ImageURL: "invite_10_frens.png", Completed: inviteCount >= 10},
		{Name: "Invite Bonus", Description: "邀请奖金", ImageURL: "invite_bonus.png", Completed: false}, // 假设未完成
		{Name: "Join Channel", Description: "加入频道", ImageURL: "join_channel.png", Completed: user.JoinedDiscord},
		{Name: "Follow us on X", Description: "在X上关注我们", ImageURL: "follow_us_on_x.png", Completed: user.JoinedX},
		{Name: "Join Telegram Channel", Description: "加入Telegram频道", ImageURL: "join_telegram_channel.png", Completed: user.JoinedTelegram},    // 假设已完成
		{Name: "Join Telegram group", Description: "加入Telegram频道", ImageURL: "join_telegram_channel.png", Completed: user.JoinedTelegramGroup}, // 假设已完成

	}

	// 返回任务列表
	errorss.JsonSuccess(c, tasks)
}

// GetFreeTasks 获取用户的任务状态
func GetFreeTasks(c *gin.Context) {
	userID := c.Query("user_id")

	// 在数据库中查找用户
	var user models.User
	if err := daos.DB.Where("user_id = ?", userID).First(&user).Error; err != nil {
		errorss.HandleError(c, http.StatusNotFound, err) // 用户未找到
		return
	}

	// 查询用户邀请一级数量
	var inviteCount int64
	if err := daos.DB.Model(&models.Invitation{}).Where("inviter_id = ? AND level = ?", userID, 1).Count(&inviteCount).Error; err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err) // 无法查询邀请数量
		return
	}

	// 检查是否已完成“Invite 10 Frens”任务，并奖励用户
	if inviteCount >= 10 {
		var existingReward models.AchievementReward
		if err := daos.DB.Where("user_id = ? AND achievement_name = ?", userID, "10Friend").First(&existingReward).Error; err != nil {
			// 如果没有记录，则发放奖励
			if err := IncrementBalance(userID, 1000); err != nil {
				errorss.HandleError(c, http.StatusInternalServerError, err) // Redis 更新失败
				return
			}

			// 创建奖励记录
			reward := models.AchievementReward{
				UserID:          userID,
				AchievementName: "10Friend",
				RewardType:      "Balance",
				Amount:          1000,
				CreatedAt:       time.Now(),
			}

			if err := daos.DB.Create(&reward).Error; err != nil {
				errorss.HandleError(c, http.StatusInternalServerError, err) // 数据库插入失败
				return
			}
		}
	}

	// 定义任务列表
	tasks := []RegularTask{
		{Name: "Invite a Frens", Description: "500 points + 3 Free Card", ImageURL: "invite_10_frens.png", Completed: inviteCount >= 10},
		{Name: "Invite a Premium Fren", Description: "2500 points + 15 Free Card", ImageURL: "invite_bonus.png", Completed: false}, // 假设未完成
	}

	// 返回任务列表
	errorss.JsonSuccess(c, gin.H{"tasks": tasks})
}
func GetBoostTasks(c *gin.Context) {
	userID := c.Query("user_id")

	// 获取今天的时间范围
	todayStart := time.Now().Truncate(24 * time.Hour)
	todayEnd := todayStart.Add(24 * time.Hour)

	// 计算今日剩余可领取次数
	limit := int64(20) // 查询今日领取的任务次数
	var dailyCount int64
	err := daos.DB.Model(&models.FreeCardTask{}).
		Where("user_id = ? AND created_at BETWEEN ? AND ? AND is_granted = ? ", userID, todayStart, todayEnd, true).
		Count(&dailyCount).Error
	if err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	remainingTasks := limit - dailyCount

	fmt.Println(remainingTasks)

	fmt.Println(dailyCount)

	// 查询下一次免费卡片发放时间
	var nextReleaseTime time.Time
	err = daos.DB.Model(&models.FreeCardTask{}).
		Where("user_id = ? AND is_granted = ?", userID, false).
		Order("created_at ASC").
		Pluck("granted_at", &nextReleaseTime).Error
	if err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	//// 设置下一次发放时间为时间戳格式
	var nextReleaseTimestamp int64

	nextReleaseTimestamp = nextReleaseTime.Unix()
	//	nextReleaseTimestamp = nextReleaseTime.Unix()
	//	if nextReleaseTimestamp == -62135596800 {
	//		nextReleaseTimestamp = 0
	//	}
	//} else {
	//	nextReleaseTimestamp = 0
	//}
	//
	// 构建任务状态信息
	tasks := map[string]interface{}{
		"daily_remaining_tasks": map[string]interface{}{
			"name":        "daily_remaining_tasks",
			"description": strconv.FormatInt(dailyCount+5, 10) + "/20 " + "available",
			"value":       strconv.FormatInt(dailyCount, 10),
			"url":         "http://example.com/daily_remaining_tasks", // 可替换为实际的URL
			"completed":   dailyCount >= 5,
		},
		"next_release_time": map[string]interface{}{
			"name":        "next_release_time",
			"description": "Next Free Card",
			"value":       nextReleaseTimestamp,
			"url":         "http://example.com/next_release_time", // 可替换为实际的URL
			"completed":   false,
		},
	}

	errorss.JsonSuccess(c, tasks)
}

func UserLoginTriggered(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		errorss.HandleError(c, http.StatusBadRequest, fmt.Errorf("user_id is required"))
		return
	}

	var user models.User
	if err := daos.DB.Where("user_id = ?", userID).First(&user).Error; err != nil {
		errorss.HandleError(c, http.StatusNotFound, err) // 用户未找到
		return
	}

	// 获取今天的时间范围
	todayStart := time.Now().Truncate(24 * time.Hour)
	todayEnd := todayStart.Add(24 * time.Hour)

	// 查询今日未领取的任务次数
	var count int64
	err := daos.DB.Model(&models.FreeCardTask{}).
		Where("user_id = ? AND created_at BETWEEN ? AND ?", userID, todayStart, todayEnd).
		Count(&count).Error
	if err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	if count <= int64(0) {
		user.CardCount += 5
		if err := daos.DB.Save(&user).Error; err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, err)
			return
		}

		// 缓存用户的卡片数量到 Redis
		balanceKey := user.UserID + "_card_count"
		err = configs.Rdb.Set(configs.Ctx, balanceKey, user.CardCount, 0).Err()
		if err != nil {
			log.Println("Failed to cache user card count to Redis:", err)
		}

	}

	// 检查今日发放次数是否超过限制（假设限制为 15 次）
	limit := 15
	if count >= int64(limit) {
		errorss.HandleError(c, http.StatusTooManyRequests, fmt.Errorf("Daily limit exceeded"))
		return
	}

	// 查询今天的最后一个任务的结束时间
	var lastTaskEndTime *time.Time
	err = daos.DB.Model(&models.FreeCardTask{}).
		Where("user_id = ? AND created_at BETWEEN ? AND ?", userID, todayStart, todayEnd).
		Order("granted_at DESC").
		Pluck("granted_at", &lastTaskEndTime).Error
	if err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	if lastTaskEndTime != nil {
		fmt.Printf("Last task end time: %s\n", lastTaskEndTime.Format(time.RFC3339))
	}

	// 检查新任务的创建时间是否在上一个任务的结束时间之后
	now := time.Now()
	if lastTaskEndTime != nil && now.Before(*lastTaskEndTime) {
		errorss.HandleError(c, http.StatusBadRequest, fmt.Errorf("New task cannot be created before the last task's end time"))
		return
	}

	// 计算 GrantedAt 时间并转换为 *time.Time
	grantedAt := time.Now().Add(10 * time.Minute)
	grantedAtPtr := &grantedAt

	// 添加免费卡片任务记录
	task := models.FreeCardTask{
		UserID:    userID,
		CreatedAt: time.Now(),
		GrantedAt: grantedAtPtr,
		IsGranted: false,
	}

	err = daos.DB.Create(&task).Error
	if err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	// 返回成功消息
	errorss.JsonSuccess(c, gin.H{"message": "Free card task successfully created"})
}
func GetInvitationList(c *gin.Context) {
	userID := c.Query("user_id")

	// Validate input
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	// Get level 1 invitations (direct invitations)
	var level1Invitations []models.Invitation
	err := daos.DB.Where("inviter_id = ? AND level = ?", userID, 1).Pluck("invitee_address", &level1Invitations).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get level 2 invitations (invitations from level 1 invitees)
	var level2Invitations []models.Invitation
	var level1UserIDs []string
	for _, invite := range level1Invitations {
		level1UserIDs = append(level1UserIDs, invite.InviteeUserID)
	}

	err = daos.DB.Where("inviter_id IN (?) AND level = ?", level1UserIDs, 2).Pluck("invitee_address", &level2Invitations).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Prepare response
	type InvitationResponse struct {
		Address string `json:"address"`
	}

	var allInvitations []InvitationResponse
	for _, invite := range level1Invitations {
		allInvitations = append(allInvitations, InvitationResponse{
			Address: invite.InviteeAddress,
		})
	}

	for _, invite := range level2Invitations {
		allInvitations = append(allInvitations, InvitationResponse{
			Address: invite.InviteeAddress,
		})
	}

	// Return the result
	c.JSON(http.StatusOK, gin.H{
		"invitations": allInvitations,
	})
}

// 绑定
// BindUserAddress 处理用户地址绑定的请求
func BindUserAddress(c *gin.Context) {
	var input struct {
		UserID  string `json:"userid" binding:"required"`
		Address string `json:"address" binding:"required"` // 添加玩法参数
	}

	// 绑定 JSON 输入到结构体
	if err := c.ShouldBindJSON(&input); err != nil {
		errorss.HandleError(c, 400, errors.New("Invalid JSON input"))
		return
	}

	// 在数据库中查找用户
	var user models.User
	if err := daos.DB.Where("user_id = ?", input.UserID).First(&user).Error; err != nil {
		errorss.HandleError(c, 404, errors.New("User not found"))
		return
	}

	// 更新用户地址
	user.Address = input.Address
	if err := daos.DB.Save(&user).Error; err != nil {
		errorss.HandleError(c, 500, errors.New("Unable to update user address"))
		return
	}

	// 返回成功信息
	errorss.JsonSuccess(c, gin.H{"message": "User address binding successful", "user": user})
}

// RecordAchievement 奖励记录
func RecordAchievement(userID, achievementName, rewardType string, amount int64) error {
	reward := models.AchievementReward{
		UserID:          userID,
		AchievementName: achievementName,
		RewardType:      rewardType,
		Amount:          amount,
		CreatedAt:       time.Now(),
	}

	return daos.DB.Create(&reward).Error
}

type Order struct {
	UserID  string  `json:"user_id"` //用户
	Address string  `json:"address"` //用户地址
	Amount  float64 `json:"amount"`  //数量
	Hash    string  `json:"hash"`    //哈希
}

func CreateOrder(c *gin.Context) {
	var order Order
	if err := c.BindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	orders := models.Order{
		UserID:          order.UserID,
		Address:         order.Address,
		Status:          "pending",
		Amount:          order.Amount,
		TransactionHash: order.Hash,
		CreatedAt:       time.Now(),
	}

	// 保存到数据库
	if err := daos.DB.Create(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
		return
	}

	errorss.JsonSuccess(c, gin.H{"message": "Order created successfully", "order": orders})

}

// ShareTaskCompletion 处理分享任务完成的请求
func ShareTaskCompletion(c *gin.Context) {
	var input struct {
		UserID string `json:"userid" binding:"required"`
		Type   string `json:"type" binding:"required"` // 任务类型: "discord", "x", "telegram"
	}

	// 绑定 JSON 输入到结构体
	if err := c.ShouldBindJSON(&input); err != nil {
		errorss.HandleError(c, http.StatusBadRequest, errors.New("Invalid JSON input"))
		return
	}

	// 在数据库中查找用户
	var user models.User
	if err := daos.DB.Where("user_id = ?", input.UserID).First(&user).Error; err != nil {
		errorss.HandleError(c, http.StatusNotFound, errors.New("User not found"))
		return
	}
	// 从数据库中读取 APP 配置
	var appConfig models.APP
	if err := daos.DB.First(&appConfig).Error; err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to retrieve app configuration"))
		return
	}

	// 处理任务完成状态
	switch input.Type {
	case "discord":
		// 检查是否已有奖励记录
		var existingReward models.AchievementReward
		if err := daos.DB.Where("user_id = ? AND achievement_name = ?", input.UserID, "discord").First(&existingReward).Error; err == nil {
			// 奖励记录已存在
			errorss.HandleError(c, http.StatusOK, errors.New("Reward already granted for this achievement"))
			return
		}
		user.JoinedDiscord = true
		// 保存更新后的用户信息
		if err := daos.DB.Save(&user).Error; err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to update user information"))
			return
		}
		// 增加用户余额
		if err := IncrementBalance(input.UserID, appConfig.DiscordAmount); err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to update user balance"))
			return
		}
		// 记录奖励
		if err := RecordAchievement(input.UserID, "discord", "Balance", appConfig.DiscordAmount); err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to record achievement"))
			return
		}

	case "x":
		// 检查是否已有奖励记录
		var existingReward models.AchievementReward
		if err := daos.DB.Where("user_id = ? AND achievement_name = ?", input.UserID, "x").First(&existingReward).Error; err == nil {
			// 奖励记录已存在
			errorss.HandleError(c, http.StatusOK, errors.New("Reward already granted for this achievement"))
			return
		}
		user.JoinedX = true
		// 保存更新后的用户信息
		if err := daos.DB.Save(&user).Error; err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to update user information"))
			return
		}
		// 增加用户余额
		if err := IncrementBalance(input.UserID, appConfig.TwitterAmount); err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to update user balance"))
			return
		}
		// 记录奖励
		if err := RecordAchievement(input.UserID, "x", "Balance", appConfig.TwitterAmount); err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to record achievement"))
			return
		}

	case "telegram":
		// 检查是否已有奖励记录
		var existingReward models.AchievementReward
		if err := daos.DB.Where("user_id = ? AND achievement_name = ?", input.UserID, "telegram").First(&existingReward).Error; err == nil {
			// 奖励记录已存在
			errorss.HandleError(c, http.StatusOK, errors.New("Reward already granted for this achievement"))
			return
		}
		user.JoinedTelegram = true
		// 保存更新后的用户信息
		if err := daos.DB.Save(&user).Error; err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to update user information"))
			return
		}
		// 增加用户余额
		if err := IncrementBalance(input.UserID, appConfig.TelegramGroupAmount); err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to update user balance"))
			return
		}

		// 记录奖励
		if err := RecordAchievement(input.UserID, "telegram", "Balance", appConfig.TelegramChannelAmount); err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to record achievement"))
			return
		}

	case "telegramGroup":
		// 检查是否已有奖励记录
		var existingReward models.AchievementReward
		if err := daos.DB.Where("user_id = ? AND achievement_name = ?", input.UserID, "telegramGroup").First(&existingReward).Error; err == nil {
			// 奖励记录已存在
			errorss.HandleError(c, http.StatusOK, errors.New("Reward already granted for this achievement"))
			return
		}
		user.JoinedTelegramGroup = true
		// 保存更新后的用户信息
		if err := daos.DB.Save(&user).Error; err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to update user information"))
			return
		}
		// 增加用户余额
		if err := IncrementBalance(input.UserID, appConfig.TelegramGroupAmount); err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to update user balance"))
			return
		}
		// 记录奖励
		if err := RecordAchievement(input.UserID, "telegramGroup", "Balance", appConfig.TelegramGroupAmount); err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to record achievement"))
			return
		}

	default:
		errorss.HandleError(c, http.StatusBadRequest, errors.New("Invalid task type"))
		return
	}

	user.UpdatedAt = time.Now()

	// 返回成功信息
	errorss.JsonSuccess(c, gin.H{"message": "Task completion status updated successfully and balance increased by 10000", "user": user})
}

func GetTypeList(c *gin.Context) {
	var cardTypes []models.CardType

	userID := c.Query("userid")
	if userID == "" {
		errorss.HandleError(c, http.StatusBadRequest, errors.New("userid is required"))
		return
	}

	// 查询数据库中的所有 CardType
	if err := daos.DB.Find(&cardTypes).Error; err != nil {
		// 如果查询出现错误，返回 HTTP 500 错误
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve card types"})
		return
	}

	// 从数据库中检索用户信息
	var user models.User
	if err := daos.DB.Where("user_id = ?", userID).First(&user).Error; err != nil {
		errorss.HandleError(c, http.StatusNotFound, err) // 用户未找到
		return
	}

	// 获取每日购买限制
	var appConfig models.APP
	if err := daos.DB.First(&appConfig).Error; err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	// 创建一个新的结构体来包含 CardType 和 CanPurchase 字段
	type CardTypeWithPurchaseStatus struct {
		models.CardType
		CanPurchase bool `json:"can_purchase"`
	}

	var cardTypesWithStatus []CardTypeWithPurchaseStatus

	// 遍历 cardTypes 并检查用户的购买限制
	for _, cardType := range cardTypes {
		reachedLimit, err := hasReachedPurchaseLimit(userID, appConfig.DailyCardPurchaseLimit, cardType.Type)
		if err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, err) // 查询购买记录失败
			return
		}

		cardTypeWithStatus := CardTypeWithPurchaseStatus{
			CardType:    cardType,
			CanPurchase: !reachedLimit,
		}

		cardTypesWithStatus = append(cardTypesWithStatus, cardTypeWithStatus)
	}

	// 返回成功信息
	errorss.JsonSuccess(c, cardTypesWithStatus)
}

// GetPrizes handles the request to fetch all prizes.
func GetPrizes(c *gin.Context) {
	var prizes []models.Prize
	if err := daos.DB.Find(&prizes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch prizes"})
		return
	}
	c.JSON(http.StatusOK, prizes)
}

// UpdatePrize 处理奖品更新
func UpdatePrize(c *gin.Context) {
	var updateData models.Prize

	// 从请求体中获取更新的数据
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// 根据 IsTimeBased 的值决定是否清空时间字段
	updates := map[string]interface{}{
		"name":                updateData.Name,
		"type":                updateData.Type,
		"value":               updateData.Value,
		"probability":         updateData.Probability,
		"is_time_based":       updateData.IsTimeBased,
		"play_mode":           updateData.PlayMode,
		"is_auto_distributed": updateData.IsAutoDistributed,
		"quota":               updateData.Quota,
		"image_url":           updateData.ImageURL,
	}

	// 如果 IsTimeBased 为 false，清空时间字段
	if !updateData.IsTimeBased {
		updates["start_time"] = nil
		updates["end_time"] = nil
	} else {
		updates["start_time"] = updateData.StartTime
		updates["end_time"] = updateData.EndTime
	}

	// 执行更新操作
	if err := daos.DB.Model(&models.Prize{}).Where("id = ?", updateData.ID).
		Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update prize"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Prize updated successfully"})
}

// GetPrizeList 获取奖品列表
func GetPrizeList(c *gin.Context) {
	mode := c.Query("mode")

	var exhibitions []models.Exhibition

	// 根据 mode 查询不同的展览数据
	switch mode {
	case "1":
		// 查询某种条件下的展览数据（这里的条件根据实际需求调整）
		if err := daos.DB.Where("play_mode = ?", "1").Find(&exhibitions).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": "Failed to fetch exhibitions"})
			return
		}

	case "2":
		// 查询某种条件下的展览数据（这里的条件根据实际需求调整）
		if err := daos.DB.Where("play_mode = ?", "2").Find(&exhibitions).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": "Failed to fetch exhibitions"})
			return
		}

	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": "Invalid mode"})
		return
	}

	// 格式化返回的数据
	response := []gin.H{}
	for _, exhibition := range exhibitions {
		response = append(response, gin.H{
			"id":   exhibition.Number,
			"name": exhibition.Name,
			"icon": exhibition.ImageURL,
		})
	}

	// 返回成功信息
	errorss.JsonSuccess(c, response)
}
