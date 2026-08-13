package config

import (
	"os"
	"testing"
)

// TestLoadSecretFromEnv 验证环境变量优先级高于配置文件默认值。
// 对齐 12-factor：所有可变配置经环境变量注入，部署无需 envsubst 渲染。
func TestLoadSecretFromEnv(t *testing.T) {
	// 排列：env 变量名 → 设置值 → 期望覆盖到的配置字段
	cases := []struct {
		name, envKey, envVal, expectField string
		set                               func(c *Config) string
	}{
		{"JWT", "CSCAN_JWT_SECRET", "jwt-abc", "AccessSecret", func(c *Config) string { return c.Auth.AccessSecret }},
		{"MongoURI", "CSCAN_MONGO_URI", "mongodb://u:p@host:27017/db?authSource=admin", "Mongo.Uri", func(c *Config) string { return c.Mongo.Uri }},
		{"MongoDB", "CSCAN_MONGO_DB", "custom-db", "Mongo.DbName", func(c *Config) string { return c.Mongo.DbName }},
		{"RedisHost", "CSCAN_REDIS_HOST", "redis:6379", "Redis.Host", func(c *Config) string { return c.Redis.Host }},
		{"RedisPass", "CSCAN_REDIS_PASSWORD", "secret-pass", "Redis.Pass", func(c *Config) string { return c.Redis.Pass }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.envKey, tc.envVal)
			c := Config{}
			c.LoadSecretFromEnv()
			if got := tc.set(&c); got != tc.envVal {
				t.Fatalf("%s: expected %q, got %q", tc.name, tc.envVal, got)
			}
		})
	}
}

// TestLoadSecretFromEnv_EmptyKeepsDefault 验证环境变量为空时不覆盖配置默认值。
func TestLoadSecretFromEnv_EmptyKeepsDefault(t *testing.T) {
	// 确保关键 env 不存在
	for _, k := range []string{"CSCAN_JWT_SECRET", "CSCAN_MONGO_URI", "CSCAN_MONGO_DB", "CSCAN_REDIS_HOST", "CSCAN_REDIS_PASSWORD"} {
		os.Unsetenv(k)
	}
	c := Config{}
	c.Auth.AccessSecret = "preset-jwt"
	c.Mongo.Uri = "mongodb://preset:27017"
	c.Mongo.DbName = "preset-db"
	c.Redis.Host = "preset-redis:6379"
	c.Redis.Pass = "preset-pass"

	c.LoadSecretFromEnv()

	if c.Auth.AccessSecret != "preset-jwt" {
		t.Fatalf("AccessSecret overwritten: %q", c.Auth.AccessSecret)
	}
	if c.Mongo.Uri != "mongodb://preset:27017" {
		t.Fatalf("Mongo.Uri overwritten: %q", c.Mongo.Uri)
	}
	if c.Mongo.DbName != "preset-db" {
		t.Fatalf("Mongo.DbName overwritten: %q", c.Mongo.DbName)
	}
	if c.Redis.Host != "preset-redis:6379" {
		t.Fatalf("Redis.Host overwritten: %q", c.Redis.Host)
	}
	if c.Redis.Pass != "preset-pass" {
		t.Fatalf("Redis.Pass overwritten: %q", c.Redis.Pass)
	}
}
