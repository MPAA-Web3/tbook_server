package main

import (
	"github.com/beego/beego/v2/core/logs"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"tbooks/configs"
	"tbooks/daos"
	"tbooks/handle"
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
	log.Println("Database connected successfully")
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
		public.GET("/ping", handle.GetPing)      // 不鉴权的测试接口 ✅
		public.GET("/luckDraw", handle.LuckDraw) // 抽奖
		public.GET("/userBalance", handle.UserBalance)

	}

	//// 鉴权接口
	//private := r.Group("/api/v1")
	//private.Use(handle.AuthMiddleware()) // 启用鉴权中间件x
	//{
	//
	//}
}
