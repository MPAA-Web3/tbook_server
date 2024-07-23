package handle

import (
	"github.com/gin-gonic/gin"
)

func GetPing(c *gin.Context) {
	//wallet := models.User{Name: "123"}
	//err := daos.AddUser(wallet)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}

	//address, ok := GetAddressFromContext(c)
	//if !ok {
	//	return // 错误信息已经在 GetAddressFromContext 中处理
	//}
	c.String(200, "pong")
}
