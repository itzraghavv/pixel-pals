package platform

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port, DatabaseURL, RedisURL, JWTSecret, Env                           string
	Origins                                                               []string
	Compat, AllowAdmin                                                    bool
	S3Endpoint, S3Region, S3Bucket, S3AccessKey, S3SecretKey, S3PublicURL string
}

func Load() (Config, error) {
	return load("HTTP_PORT", "8080")
}

type RealtimeConfig struct {
	Config
	AllowOverlap bool
}

func LoadRealtime() (RealtimeConfig, error) {
	base, err := load("REALTIME_PORT", "8081")
	c := RealtimeConfig{Config: base}
	if err != nil {
		return c, err
	}
	c.AllowOverlap, err = strconv.ParseBool(env("ALLOW_TILE_OVERLAP", "true"))
	if err != nil {
		return c, fmt.Errorf("ALLOW_TILE_OVERLAP: %w", err)
	}
	return c, nil
}

func load(portKey, defaultPort string) (Config, error) {
	c := Config{Port: env(portKey, defaultPort), DatabaseURL: os.Getenv("DATABASE_URL"), RedisURL: os.Getenv("REDIS_URL"), JWTSecret: os.Getenv("JWT_SECRET"), Env: env("APP_ENV", "production"), Origins: strings.FieldsFunc(os.Getenv("CORS_ORIGINS"), func(r rune) bool { return r == ',' }), S3Endpoint: os.Getenv("S3_ENDPOINT"), S3Region: env("S3_REGION", "us-east-1"), S3Bucket: os.Getenv("S3_BUCKET"), S3AccessKey: os.Getenv("S3_ACCESS_KEY"), S3SecretKey: os.Getenv("S3_SECRET_KEY"), S3PublicURL: os.Getenv("S3_PUBLIC_URL")}
	var err error
	if c.Compat, err = strconv.ParseBool(env("COMPAT_V0", "false")); err != nil {
		return c, fmt.Errorf("COMPAT_V0: %w", err)
	}
	if c.AllowAdmin, err = strconv.ParseBool(env("ALLOW_ADMIN_SIGNUP", "false")); err != nil {
		return c, fmt.Errorf("ALLOW_ADMIN_SIGNUP: %w", err)
	}
	if c.AllowAdmin && c.Env != "development" {
		return c, fmt.Errorf("ALLOW_ADMIN_SIGNUP requires APP_ENV=development")
	}
	if len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("JWT_SECRET must have at least 32 bytes")
	}
	for name, value := range map[string]string{"DATABASE_URL": c.DatabaseURL, "REDIS_URL": c.RedisURL} {
		u, e := url.Parse(value)
		if e != nil || u.Host == "" {
			return c, fmt.Errorf("%s must be a connection URL", name)
		}
	}
	port, e := strconv.Atoi(c.Port)
	if e != nil || port < 1 || port > 65535 {
		return c, fmt.Errorf("invalid %s", portKey)
	}
	for i, o := range c.Origins {
		c.Origins[i] = strings.TrimSpace(o)
		u, e := url.Parse(c.Origins[i])
		if e != nil || u.Host == "" || u.Scheme != "http" && u.Scheme != "https" {
			return c, fmt.Errorf("invalid CORS_ORIGINS")
		}
	}
	return c, nil
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
