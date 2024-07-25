package main

import (
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/core/logs"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"io/ioutil"
	"log"
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

// ReadABIFromFile 读取 ABI 文件内容
func ReadABIFromFile(filePath string) (string, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func main() {
	initLogger() // 初始化日志
	configs.Config()
	configs.ParseConfig("./configs/config.yaml") // 加载 configs 目录中的配置文件
	daos.InitMysql()
	configs.NewRedis()
	// 启动定时任务
	go startUserCacheJob()
	r := gin.Default()
	route(r)
	r.Use(handle.Core())
	err := r.Run(":" + configs.Config().Port)
	if err != nil {
		return
	}
}

func route(r *gin.Engine) {
	// 不鉴权接口
	public := r.Group("/api/v1")
	{
		public.GET("/ping", handle.GetPing)             // 不鉴权的测试接口 ✅
		public.POST("/luckDraw", handle.LuckDraw)       // 抽奖
		public.POST("/userBalance", handle.UserBalance) //用户的余额
		public.POST("/createUser", handle.CreateUser)   //创建用户
		public.POST("/buyCard", handle.BuyCard)
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
