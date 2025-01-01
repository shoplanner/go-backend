package config

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sethvargo/go-envconfig"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Service ListenerCfg `yaml:"listener"`
	Auth    AuthCfg     `yaml:"auth"`
}

type ListenerCfg struct {
	Net  string `yaml:"net"`
	Host string `yaml:"host"`
	Port uint16 `yaml:"port"`
}

type AuthCfg struct {
	RefreshTokenLiveTime time.Duration `yaml:"refresh_token_livetime"`
	AccessTokenLiveTime  time.Duration `yaml:"access_token_livetime"`
}

type Env struct {
	Database DatabaseEnv
	Redis    RedisEnv
	Auth     AuthEnv
}

type AuthEnv struct {
	PrivateKey string `env:"AUTH_PRIVATE_KEY" json:"-"`
}

type RedisEnv struct {
	Addr     string `env:"REDIS_ADDR"`
	Password string `env:"REDIS_PASS" json:"-"`
	Net      string `env:"REDIS_NET"`
	User     string `env:"REDIS_USER" json:"-"`
}

type DatabaseEnv struct {
	Password string `env:"DATABASE_PASSWORD" json:"-"`
	Host     string `env:"DATABASE_HOST,required"`
	User     string `env:"DATABASE_USER" json:"-"`
	Name     string `env:"DATABASE_NAME"`
	Net      string `env:"DATABASE_NET"`
}

func ParseConfig(path string) (Config, error) {
	var cfg Config

	content, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("can't open config file '%s': %w", path, err)
	}

	if err = yaml.Unmarshal(content, &cfg); err != nil {
		return cfg, fmt.Errorf("can't decode yaml in file '%s': %w", path, err)
	}

	return cfg, nil
}

func ParseEnv(ctx context.Context) (Env, error) {
	var env Env

	if err := envconfig.Process(ctx, &env); err != nil {
		return env, fmt.Errorf("can't load config from env: %w", err)
	}

	return env, nil
}
