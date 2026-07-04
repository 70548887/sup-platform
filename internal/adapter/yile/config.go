package yile

import "os"

// Config 亿乐适配器配置
type Config struct {
	BaseURL   string // 亿乐API基础地址
	AppId     string // 亿乐AppId
	AppSecret string // 亿乐AppSecret
}

// LoadFromEnv 从环境变量加载配置
func LoadFromEnv() *Config {
	return &Config{
		BaseURL:   getEnv("YILE_API_URL", "https://api.yile.com"),
		AppId:     getEnv("YILE_APP_ID", ""),
		AppSecret: getEnv("YILE_APP_SECRET", ""),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
