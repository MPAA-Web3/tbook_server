package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/core/logs"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"log"
	"strconv"
	"tbooks/configs"
	"tbooks/daos"
	"tbooks/handle"
	"tbooks/models"
	"time"
)

// 初始化日志
func initLogger() {
	err := logs.SetLogger(logs.AdapterFile, `{"filename":"logs/mpaa.log","level":7,"maxlines":0,"maxsize":0,"daily":true,"maxdays":10,"color":true}`)
	if err != nil {
		return
	}
}

func main() {
	initLogger() // 初始化日志
	configs.Config()
	configs.ParseConfig("./configs/config.yaml") // 加载 configs 目录中的配置文件
	daos.InitMysql()
	configs.NewRedis()
	// 启动定时任务
	go startUserCacheJob()
	go startFreeCardTaskJob()
	// 创建 Cron 调度器
	c := cron.New(cron.WithLocation(time.UTC))

	// 添加每天午夜（0:00）执行的任务
	_, err := c.AddFunc("0 0 * * *", func() {
		addPrizeSlots()
	})
	if err != nil {
		log.Fatalf("Failed to add cron job for addPrizeSlots: %v", err)
	}
	// 启动 Cron 调度器
	c.Start()
	// 让主 goroutine 运行，以保持程序不退出
	select {}
	r := gin.Default()
	route(r)
	r.Use(handle.Core())
	err = r.Run(":" + configs.Config().Port)
	if err != nil {
		return
	}
}

func route(r *gin.Engine) {
	// 不鉴权接口
	public := r.Group("/api/v1")
	{
		public.GET("/ping", handle.GetPing)                          // 不鉴权的测试接口 ✅
		public.POST("/luckDraw", handle.LuckDraw)                    // 抽奖
		public.POST("/userBalance", handle.UserBalance)              //用户的余额
		public.POST("/createUser", handle.CreateUser)                //创建用户
		public.POST("/buyCard", handle.BuyCard)                      //购买卡片
		public.GET("/getLeaderboard", handle.GetLeaderboard)         //获取排行榜
		public.GET("/getRegularTasks", handle.GetRegularTasks)       //获取日常任务
		public.GET("/getBoostTasks", handle.GetBoostTasks)           //获取Boost任务
		public.GET("/userLoginTriggered", handle.UserLoginTriggered) //用户登陆触发
		public.GET("/getFreeTasks", handle.GetFreeTasks)             //获取用户任务
		//public.GET("/getInvitationList", handle.GetInvitationList)      //获取邀请列表
		public.POST("/bindUserAddress", handle.BindUserAddress)         //绑定用户地址
		public.POST("/shareTaskCompletion", handle.ShareTaskCompletion) //分享任务完成
		public.POST("/createOrder", handle.CreateOrder)
	}

	//// 鉴权接口
	//private := r.Group("/api/v1")
	//private.Use(handle.AuthMiddleware()) // 启用鉴权中间件x
	//{
	//
	//}ç
}

func startUserCacheJob() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cacheUserData()
		}
	}
}

func startFreeCardTaskJob() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			processFreeCardTasks()
		}
	}
}

func cacheUserData() {
	var users []models.User
	if err := daos.DB.Find(&users).Error; err != nil {
		log.Println("Failed to load user data from MySQL:", err)
		return
	}

	for _, user := range users {
		userKey := "user:" + user.UserID
		cardCountKey := user.UserID + "_card_count"
		balanceKey := user.UserID + "_balance"

		// 检查 Redis 中是否已经存在数据
		redisUserJSON, err := configs.Rdb.Get(configs.Ctx, userKey).Result()
		if err == redis.Nil {
			// 如果 Redis 中没有数据，则缓存 MySQL 中的数据到 Redis
			userJSON, err := json.Marshal(user)
			if err != nil {
				log.Println("Failed to marshal user data:", err)
				continue
			}
			fmt.Printf("Caching user data: %s\n", string(userJSON))

			// 存储用户的完整信息
			err = configs.Rdb.Set(configs.Ctx, userKey, userJSON, 0).Err()
			if err != nil {
				log.Println("Failed to cache user data to Redis:", err)
			}

			// 存储用户的卡片数量
			err = configs.Rdb.Set(configs.Ctx, cardCountKey, user.CardCount, 0).Err()
			if err != nil {
				log.Println("Failed to cache user card count to Redis:", err)
			}

			// 存储用户的余额
			err = configs.Rdb.Set(configs.Ctx, balanceKey, user.Balance, 0).Err()
			if err != nil {
				log.Println("Failed to cache user balance to Redis:", err)
			}

		} else if err != nil {
			log.Println("Failed to get user data from Redis:", err)
			continue
		} else {
			// 如果 Redis 中已经存在数据，检查和更新数据库
			var redisUser models.User
			if err := json.Unmarshal([]byte(redisUserJSON), &redisUser); err != nil {
				log.Println("Failed to unmarshal Redis user data:", err)
				continue
			}

			// 检查 Redis 中的卡片数量和余额
			redisCardCount, err := configs.Rdb.Get(configs.Ctx, cardCountKey).Int()
			if err != nil && err != redis.Nil {
				log.Println("Failed to get card count from Redis:", err)
				continue
			}

			redisBalance, err := configs.Rdb.Get(configs.Ctx, balanceKey).Float64()
			if err != nil && err != redis.Nil {
				log.Println("Failed to get balance from Redis:", err)
				continue
			}

			// 如果 Redis 中的卡片数量和余额与 MySQL 中的数据不同，则更新 MySQL
			if redisCardCount != user.CardCount || redisBalance != user.Balance {
				user.CardCount = redisCardCount
				user.Balance = redisBalance
				if err := daos.DB.Save(&user).Error; err != nil {
					log.Println("Failed to update user data in MySQL:", err)
				}
			}
		}
	}
	log.Println("User data cached to Redis successfully")
}

func processFreeCardTasks() {
	// 查询所有未发放且当前时间已经超过granted_at时间的任务
	var tasks []models.FreeCardTask
	err := daos.DB.Model(&models.FreeCardTask{}).
		Where("is_granted = ? AND granted_at <= ?", false, time.Now().Add(-10*time.Second)).
		Find(&tasks).Error
	if err != nil {
		log.Println("Failed to query ungranted tasks:", err)
		return
	}
	for _, task := range tasks {
		// 标记任务为已发放
		task.IsGranted = true

		// 更新任务记录
		err := daos.DB.Save(&task).Error
		if err != nil {
			log.Println("Failed to update task record:", err)
			continue
		}

		// 更新用户卡片数量到 Redis
		cardCountKey := task.UserID + "_card_count"

		// 获取当前用户的卡片数量
		cardCountStr, err := configs.Rdb.Get(context.Background(), cardCountKey).Result()
		if err == redis.Nil {
			// 如果 Redis 中没有用户的卡片数量，则从数据库中获取
			var user models.User
			if err := daos.DB.Where("user_id = ?", task.UserID).First(&user).Error; err != nil {
				log.Println("Failed to get user data from MySQL:", err)
				continue
			}
			cardCountStr = string(user.CardCount)
		} else if err != nil {
			log.Println("Failed to get card count from Redis:", err)
			continue
		}

		// 解析卡片数量并增加
		cardCount, err := strconv.Atoi(cardCountStr)
		if err != nil {
			log.Println("Failed to parse card count from Redis:", err)
			continue
		}
		cardCount++

		// 更新卡片数量到 Redis
		err = configs.Rdb.Set(context.Background(), cardCountKey, cardCount, 0).Err()
		if err != nil {
			log.Println("Failed to update card count in Redis:", err)
			continue
		}

		log.Printf("Card granted to user %s, updated card count to Redis successfully\n", task.UserID)
	}
}

func addPrizeSlots() {
	// 假设你要在 Redis 中设置一个奖品名额的数量
	prizeSlotKey := "prize_slots"
	prizeSlotCount := 100 // 你可以根据需要设置奖品名额的数量

	// 将奖品名额添加到 Redis 中
	err := configs.Rdb.Set(context.Background(), prizeSlotKey, prizeSlotCount, 0).Err()
	if err != nil {
		log.Println("Failed to add prize slots to Redis:", err)
		return
	}

	log.Printf("Prize slots set to %d in Redis successfully\n", prizeSlotCount)
}
