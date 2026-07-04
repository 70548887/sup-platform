package zhike

import "os"

// Config 直客适配器配置
type Config struct {
	AppId     string // 直客AppId
	AppSecret string // 直客AppSecret
	BaseURL   string // 直客API基础地址
}

// LoadFromEnv 从环境变量加载配置
func LoadFromEnv() *Config {
	return &Config{
		AppId:     getEnv("ZHIKE_APP_ID", ""),
		AppSecret: getEnv("ZHIKE_APP_SECRET", ""),
		BaseURL:   getEnv("ZHIKE_BASE_URL", "https://api.zhike.com"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
