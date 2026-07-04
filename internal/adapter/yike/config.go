package yike

import "os"

// Config 易客适配器配置
type Config struct {
	AppId     string // 易客AppId
	AppSecret string // 易客AppSecret
	BaseURL   string // 易客API基础地址
}

// LoadFromEnv 从环境变量加载配置
func LoadFromEnv() *Config {
	return &Config{
		AppId:     getEnv("YIKE_APP_ID", ""),
		AppSecret: getEnv("YIKE_APP_SECRET", ""),
		BaseURL:   getEnv("YIKE_BASE_URL", "https://api.yike.com"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
