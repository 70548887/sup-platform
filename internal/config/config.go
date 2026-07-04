package config

// Config 应用配置
type Config struct {
	App         AppConfig         `yaml:"app"`
	Database    DatabaseConfig    `yaml:"database"`
	Redis       RedisConfig       `yaml:"redis"`
	JWT         JWTConfig         `yaml:"jwt"`
	MultiTenant MultiTenantConfig `yaml:"multi_tenant"`
	Security    SecurityConfig    `yaml:"security"`
}

type AppConfig struct {
	Name string `yaml:"name"`
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"` // debug/release
}

type DatabaseConfig struct {
	Driver   string `yaml:"driver"` // mysql
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Enabled  bool   `yaml:"enabled"`
	Prefix   string `yaml:"prefix"`
}

type JWTConfig struct {
	Secret string `yaml:"secret"`
	Expire int    `yaml:"expire"` // hours
}

// MultiTenantConfig 多租户配置
type MultiTenantConfig struct {
	Enabled         bool `yaml:"enabled"`
	DefaultTenantID uint `yaml:"default_tenant_id"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
	TLSEnabled     bool     `yaml:"tls_enabled"`
	CardEncryptKey string   `yaml:"card_encrypt_key" env:"CARD_ENCRYPT_KEY"` // 32字节hex编码的AES-256密钥
}
