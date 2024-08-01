package handle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
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
		errorss.HandleError(c, 403, errors.New("Insufficient card ")) // 卡片次数不足
		return
	}
	// 扣除一次卡片次数
	if err := configs.Rdb.Decr(c, input.UserID+"_card_count").Err(); err != nil {
		errorss.HandleError(c, 500, err) // 更新卡片次数失败
		return
	}
	// 选择对应玩法的奖品
	availablePrizes, ok := prizes[input.PlayMode]
	if !ok {
		errorss.HandleError(c, 400, errors.New("Invalid PlayMode parameter")) // 无效的玩法参数
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
			"message":    "congratulations! You have won the prize!",
			"prize":      prize.Name,
			"balance":    newBalance,
			"number":     prize.ImageURL, // 包含奖品图片链接
			"card_count": cardCount - 1,
		})

	case "1card":
		// 奖品是抽奖卡
		if err := configs.Rdb.Incr(c, input.UserID+"_card_count").Err(); err != nil {
			errorss.HandleError(c, 500, err) // 更新卡片次数失败
			return
		}
		newCardCount, _ := configs.Rdb.Get(c, input.UserID+"_card_count").Int()
		errorss.JsonSuccess(c, gin.H{
			"message":    "congratulations! You have won a lottery card!",
			"prize":      prize.Name,
			"card_count": newCardCount,
			"number":     prize.ImageURL, // 包含奖品图片链接
		})

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
		UserID string `json:"userid" binding:"required"` // 用户ID，必填
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

	// 检查用户是否有足够的余额购买卡片
	if user.Balance < 100 {
		errorss.HandleError(c, http.StatusPaymentRequired, nil) // 余额不足
		return
	}

	// 扣除余额并增加卡片数量
	user.Balance -= 100
	user.CardCount += 1
	user.UpdatedAt = time.Now() // 更新最后修改时间

	// 保存更新后的用户信息到数据库
	if err := daos.DB.Save(&user).Error; err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err) // 更新用户失败
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
	errorss.JsonSuccess(c, gin.H{
		"message": "Card purchased successfully",
		"user":    user,
	})
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
		{Name: "Join Telegram Channel", Description: "加入Telegram频道", ImageURL: "join_telegram_channel.png", Completed: user.JoinedTelegram}, // 假设已完成
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
		{Name: "Invite a Frens", Description: "Earn 500", ImageURL: "invite_10_frens.png", Completed: inviteCount >= 10},
		{Name: "Invite a Premium Fren", Description: "Earn 500", ImageURL: "invite_bonus.png", Completed: false}, // 假设未完成
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
	limit := int64(5) // 查询今日领取的任务次数
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
			"description": strconv.FormatInt(dailyCount, 10) + "/5" + "available",
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

	// 获取今天的时间范围
	todayStart := time.Now().Truncate(24 * time.Hour)
	todayEnd := todayStart.Add(24 * time.Hour)

	// 查询今日未领取的任务次数
	var count int64
	err := daos.DB.Model(&models.FreeCardTask{}).
		Where("user_id = ? AND created_at BETWEEN ? AND ? AND is_granted = ?", userID, todayStart, todayEnd, false).
		Count(&count).Error
	if err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	// 检查今日发放次数是否超过限制（假设限制为 5 次）
	limit := 5
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
	UserID  string  `json:"user_id"`
	Address string  `json:"address"`
	Amount  float64 `json:"amount"`
}

func CreateOrder(c *gin.Context) {
	var order Order
	if err := c.BindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	orders := models.Order{
		UserID:    order.UserID,
		Address:   order.Address,
		Status:    "pending",
		Amount:    order.Amount,
		CreatedAt: time.Now(),
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
		// 增加用户余额
		if err := IncrementBalance(input.UserID, 10000); err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to update user balance"))
			return
		}
		// 记录奖励
		if err := RecordAchievement(input.UserID, "discord", "Balance", 10000); err != nil {
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
		// 增加用户余额
		if err := IncrementBalance(input.UserID, 10000); err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to update user balance"))
			return
		}
		// 记录奖励
		if err := RecordAchievement(input.UserID, "x", "Balance", 10000); err != nil {
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
		// 增加用户余额
		if err := IncrementBalance(input.UserID, 10000); err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to update user balance"))
			return
		}
		// 记录奖励
		if err := RecordAchievement(input.UserID, "telegram", "Balance", 10000); err != nil {
			errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to record achievement"))
			return
		}

	default:
		errorss.HandleError(c, http.StatusBadRequest, errors.New("Invalid task type"))
		return
	}

	user.UpdatedAt = time.Now()

	// 保存更新后的用户信息
	if err := daos.DB.Save(&user).Error; err != nil {
		errorss.HandleError(c, http.StatusInternalServerError, errors.New("Unable to update user information"))
		return
	}

	// 返回成功信息
	errorss.JsonSuccess(c, gin.H{"message": "Task completion status updated successfully and balance increased by 10000", "user": user})
}
