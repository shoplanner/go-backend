package config

import (
	"context"
	"fmt"
	"os"

	"github.com/sethvargo/go-envconfig"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Service ServiceCfg `yaml:"service"`
}

type ServiceCfg struct {
	Net  string `yaml:"net"`
	Host string `yaml:"host"`
	Port uint16 `yaml:"port"`
}

type Env struct {
	Database DatabaseEnv
	Redis    RedisEnv
}

type RedisEnv struct{}

type DatabaseEnv struct {
	Password string `env:"DATABASE_PASSWORD"`
	Host     string `env:"DATABASE_HOST,required"`
	User     string `env:"DATABASE_USER"`
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
