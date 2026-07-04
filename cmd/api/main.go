package main

import (
	"log"

	"github.com/70548887/sup-platform/internal/app"
)

// @title SUP聚合供货平台 API
// @version 1.0
// @description SUP平台后端API文档，覆盖管理端、租户端、供货商端、客户端全部接口
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	application, err := app.New()
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}
	if err := application.Run(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
