package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Socket   SocketConfig   `mapstructure:"socket"`
	Push     PushConfig     `mapstructure:"push"`
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

type PushConfig struct {
	Subject    string `mapstructure:"subject"`
	PublicKey  string `mapstructure:"public_key"`
	PrivateKey string `mapstructure:"private_key"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("SIGNAL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	for _, key := range []string{
		"server.client_addr", "server.admin_addr", "database.dsn", "auth.secret",
		"socket.read_buffer_size", "socket.write_buffer_size", "socket.pong_wait_sec", "socket.ping_period_sec",
		"push.subject", "push.public_key", "push.private_key",
	} {
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("bind environment %s: %w", key, err)
		}
	}
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if strings.TrimSpace(c.Database.DSN) == "" {
		return nil, fmt.Errorf("database DSN is required; set SIGNAL_DATABASE_DSN on the server")
	}
	if len(c.Auth.Secret) < 32 || strings.Contains(strings.ToLower(c.Auth.Secret), "change-this") {
		return nil, fmt.Errorf("auth secret must be at least 32 bytes and supplied through SIGNAL_AUTH_SECRET")
	}
	return &c, nil
}
