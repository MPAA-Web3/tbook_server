package handle

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"tbooks/configs"
	"tbooks/daos"
	"tbooks/models"
	"time"
)

type Prize struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	ImageURL string `json:"image_url"` // 添加图片 URL 字段
}

// 为不同玩法定义奖品
var prizes = map[string]map[string]Prize{
	"1": { // 玩法 1: 抽奖四个奖品
		"1": {Name: "100points", Value: "100", ImageURL: "1"},
		"2": {Name: "200points", Value: "200", ImageURL: "2"},
		"3": {Name: "300points", Value: "300", ImageURL: "3"},
		"4": {Name: "1card", Value: "1", ImageURL: "4"},
	},
	"2": { // 玩法 2: 大转盘八个奖品
		"1": {Name: "50points", Value: "50", ImageURL: "1"},
		"2": {Name: "100points", Value: "100", ImageURL: "2"},
		"3": {Name: "150points", Value: "150", ImageURL: "3"},
		"4": {Name: "200points", Value: "200", ImageURL: "4"},
		"5": {Name: "250points", Value: "250", ImageURL: "5"},
		"6": {Name: "300points", Value: "300", ImageURL: "6"},
		"7": {Name: "1card", Value: "1", ImageURL: "7"},
		"8": {Name: "400points", Value: "400", ImageURL: "8"},
	},
}

func LuckDraw(c *gin.Context) {
	var input struct {
		UserID   string `json:"userid" binding:"required"`
		PlayMode string `json:"playmode" binding:"required"` // 添加玩法参数
	}

	// 绑定 JSON 输入到结构体
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 JSON 输入"})
		return
	}

	// 获取用户的卡片次数
	cardCount, err := configs.Rdb.Get(c, input.UserID+"_card_count").Int()
	if err == redis.Nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户未找到"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户卡片次数失败"})
		return
	}

	// 检查卡片次数
	if cardCount <= 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "卡片次数不足"})
		return
	}

	// 扣除一次卡片次数
	if err := configs.Rdb.Decr(c, input.UserID+"_card_count").Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新卡片次数失败"})
		return
	}

	// 选择对应玩法的奖品
	availablePrizes, ok := prizes[input.PlayMode]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的玩法参数"})
		return
	}

	// 随机选择一个奖品
	rand.Seed(time.Now().UnixNano())
	prizeKey := []string{"1", "2", "3", "4", "5", "6", "7", "8"}[rand.Intn(len(availablePrizes))]
	prize := availablePrizes[prizeKey]

	// 处理奖品
	switch prize.Name {
	case "100points", "200points", "300points", "400points", "50points", "150points", "250points":
		// 奖品是余额
		balance, err := strconv.ParseFloat(prize.Value, 64)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "解析余额失败"})
			return
		}
		userBalanceKey := input.UserID + "_balance"
		if err := configs.Rdb.IncrByFloat(c, userBalanceKey, balance).Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新余额失败"})
			return
		}
		newBalance, _ := configs.Rdb.Get(c, userBalanceKey).Float64()
		c.JSON(http.StatusOK, gin.H{
			"message": "congratulations! You have won the prize!",
			"prize":   prize.Name,
			"balance": newBalance,
			"number":  prize.ImageURL, // 包含奖品图片链接
		})

	case "1card":
		// 奖品是抽奖卡
		if err := configs.Rdb.Incr(c, input.UserID+"_card_count").Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新卡片次数失败"})
			return
		}
		newCardCount, _ := configs.Rdb.Get(c, input.UserID+"_card_count").Int()
		c.JSON(http.StatusOK, gin.H{
			"message":    "congratulations! You have won a lottery card!",
			"prize":      prize.Name,
			"card_count": newCardCount,
			"number":     prize.ImageURL, // 包含奖品图片链接
		})

	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "未知奖品类型"})
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
//		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 JSON 输入"})
//		return
//	}
//
//	// 查找用户
//	var user models.User
//	if err := daos.DB.Where("user_id = ?", input.UserID).First(&user).Error; err != nil {
//		c.JSON(http.StatusNotFound, gin.H{"error": "用户未找到"})
//		return
//	}
//
//	// 检查卡片次数
//	if user.CardCount <= 0 {
//		c.JSON(http.StatusForbidden, gin.H{"error": "卡片次数不足"})
//		return
//	}
//
//	// 扣除一次卡片次数
//	user.CardCount -= 1
//	if err := daos.DB.Save(&user).Error; err != nil {
//		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户失败"})
//		return
//	}
//
//	// 选择对应玩法的奖品
//	availablePrizes, ok := prizes[input.PlayMode]
//	if !ok {
//		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的玩法参数"})
//		return
//	}
//
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
//			c.JSON(http.StatusInternalServerError, gin.H{"error": "解析余额失败"})
//			return
//		}
//		user.Balance += balance
//		if err := daos.DB.Save(&user).Error; err != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新余额失败"})
//			return
//		}
//		c.JSON(http.StatusOK, gin.H{
//			"message": "congratulations! You have won the prize!",
//			"prize":   prize.Name,
//			"balance": user.Balance,
//			"image":   prize.ImageURL, // 包含奖品图片链接
//		})
//
//	case "1card":
//		// 奖品是抽奖卡
//		user.CardCount += 1
//		if err := daos.DB.Save(&user).Error; err != nil {
//			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新卡片次数失败"})
//			return
//		}
//		c.JSON(http.StatusOK, gin.H{
//			"message":    "congratulations! You have won a lottery card!",
//			"prize":      prize.Name,
//			"card_count": user.CardCount,
//			"image":      prize.ImageURL, // 包含奖品图片链接
//		})
//
//	default:
//		c.JSON(http.StatusInternalServerError, gin.H{"error": "未知奖品类型"})
//	}
//}

func UserBalance(c *gin.Context) {
	var input struct {
		UserID string `json:"userid" binding:"required"`
	}

	// 绑定 JSON 输入到结构体
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 JSON 输入"})
		return
	}

	// 在数据库中查找用户
	var user models.User
	if err := daos.DB.Where("user_id = ?", input.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户未找到"})
		return
	}

	// 返回用户的余额和卡片次数
	c.JSON(http.StatusOK, gin.H{
		"user_id":       user.UserID,
		"balance":       user.Balance,
		"card_count":    user.CardCount,
		"profile_photo": user.ProfilePhoto,
	})
}

func BuyCard(c *gin.Context) {
	// 定义结构体以绑定请求中的 JSON 数据
	var input struct {
		UserID string `json:"userid" binding:"required"` // 用户ID，必填
	}

	// 将 JSON 请求体绑定到 input 结构体
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format", "details": err.Error()})
		return
	}

	// 从数据库中检索用户信息
	var user models.User
	if err := daos.DB.Where("user_id = ?", input.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// 检查用户是否有足够的余额购买卡片
	if user.Balance < 100 {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "Insufficient balance"})
		return
	}

	// 扣除余额并增加卡片数量
	user.Balance -= 100
	user.CardCount += 1
	user.UpdatedAt = time.Now() // 更新最后修改时间

	// 保存更新后的用户信息到数据库
	if err := daos.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user", "details": err.Error()})
		return
	}

	// 更新 Redis 缓存中的用户数据
	// 将用户数据序列化为 JSON
	userJSON, err := json.Marshal(user)
	if err != nil {
		log.Println("Failed to marshal user data:", err)
	} else {
		// 将用户数据缓存到 Redis 中
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
	c.JSON(http.StatusOK, gin.H{"message": "Card purchased successfully", "user": user})
}

func CreateUser(c *gin.Context) {
	var input struct {
		UserID       string `json:"userid" binding:"required"`
		ProfilePhoto string `json:"profile_photo"` // 添加头像字段
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
		UserID:       input.UserID,
		Balance:      0,                  // Default balance
		CardCount:    10000,              // Default card count
		ProfilePhoto: input.ProfilePhoto, // 设置头像
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := daos.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Respond with the created user
	c.JSON(http.StatusOK, gin.H{"message": "User created successfully", "user": user})
}
