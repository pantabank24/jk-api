package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// App
	AppName     string
	AppPort     string
	AppEnv      string
	CorsOrigins string
	// TrustedProxies — คั่นด้วยจุลภาค รับทั้ง IP เดี่ยวและ CIDR. เฉพาะคำขอที่ต่อเข้ามา
	// จาก IP ในลิสต์นี้เท่านั้นที่ X-Forwarded-For จะถูกเชื่อถือ
	TrustedProxies string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// JWT
	JWTSecret    string
	JWTExpiresIn string

	// Real-time gold price sidecar (tv-price-svc)
	GoldRealtimeURL string

	PublicAPIKey string
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No .env file found, using environment variables")
	}

	return &Config{
		// App
		AppName:     getEnv("APP_NAME", "jk-api"),
		AppPort:     getEnv("APP_PORT", "8080"),
		AppEnv:      getEnv("APP_ENV", "development"),
		CorsOrigins: getEnv("CORS_ORIGINS", "*"),
		// ค่าเริ่มต้นครอบ loopback กับช่วง private ทั้งหมด ซึ่งคือทุกอย่างที่ต่อเข้า
		// คอนเทนเนอร์ได้จริง — พอร์ตของ app ผูกไว้กับ 127.0.0.1 ของโฮสต์เท่านั้น
		// (docker-compose.yml) จึงไม่มีทางที่ peer จะเป็นเครื่องนอก
		TrustedProxies: getEnv("TRUSTED_PROXIES", "127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"),

		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "jk_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// JWT
		JWTSecret:    getEnv("JWT_SECRET", "your-secret-key"),
		JWTExpiresIn: getEnv("JWT_EXPIRES_IN", "24h"),

		// Real-time gold price sidecar
		GoldRealtimeURL: getEnv("GOLD_REALTIME_URL", "http://host.docker.internal:8000"),

		// Shared secret for the /api/v1/public/* read-only routes
		PublicAPIKey: getEnv("PUBLIC_API_KEY", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// TrustedProxyList splits TrustedProxies into the form fiber.Config wants,
// dropping blanks so a trailing comma in .env is not read as an empty proxy.
func (c *Config) TrustedProxyList() []string {
	out := []string{}
	for _, p := range strings.Split(c.TrustedProxies, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
