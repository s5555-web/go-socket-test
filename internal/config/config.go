package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Socket   SocketConfig   `mapstructure:"socket"`
}

type ServerConfig struct {
	ClientAddr string `mapstructure:"client_addr"`
	AdminAddr  string `mapstructure:"admin_addr"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}
type AuthConfig struct {
	Secret string `mapstructure:"secret"`
}

type SocketConfig struct {
	ReadBufferSize  int `mapstructure:"read_buffer_size"`
	WriteBufferSize int `mapstructure:"write_buffer_size"`
	PongWaitSec     int `mapstructure:"pong_wait_sec"`
	PingPeriodSec   int `mapstructure:"ping_period_sec"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &c, nil
}
