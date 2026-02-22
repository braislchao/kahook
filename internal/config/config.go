package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Auth   AuthConfig   `yaml:"auth"`
	Kafka  KafkaConfig  `yaml:"kafka"`
}

type ServerConfig struct {
	Port         int `yaml:"port"`
	ReadTimeout  int `yaml:"read_timeout"`
	WriteTimeout int `yaml:"write_timeout"`
	IdleTimeout  int `yaml:"idle_timeout"`
}

type AuthConfig struct {
	Type   string       `yaml:"type"`
	Users  []UserConfig `yaml:"users"`
	Tokens []string     `yaml:"tokens"`
}

type UserConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type KafkaConfig struct {
	Brokers          []string `yaml:"brokers"`
	SASLUsername     string   `yaml:"sasl_username"`
	SASLPassword     string   `yaml:"sasl_password"`
	SASLMechanism    string   `yaml:"sasl_mechanism"`
	SecurityProtocol string   `yaml:"security_protocol"`
	Acks             string   `yaml:"acks"`
	Retries          int      `yaml:"retries"`
	CompressionType  string   `yaml:"compression_type"`
}

func Load(configPath string) (*Config, error) {
	cfg := defaults()

	if err := loadFile(cfg, configPath); err != nil {
		return nil, err
	}

	applyEnv(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  10,
			WriteTimeout: 10,
			IdleTimeout:  60,
		},
		Auth: AuthConfig{
			Type: "none",
		},
		Kafka: KafkaConfig{
			Brokers:          []string{"localhost:9092"},
			Acks:             "all",
			Retries:          3,
			CompressionType:  "snappy",
			SASLMechanism:    "PLAIN",
			SecurityProtocol: "PLAINTEXT",
		},
	}
}

func loadFile(cfg *Config, path string) error {
	if path == "" {
		// Search default locations.
		for _, p := range []string{"config.yaml", "config/config.yaml", "/etc/kahook/config.yaml"} {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}

	if path == "" {
		return nil // No config file found — use defaults + env.
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("error parsing config file: %w", err)
	}

	return nil
}

// applyEnv overrides config fields from environment variables.
// Only non-empty env vars override the current value.
func applyEnv(cfg *Config) {
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = n
		}
	}
	if v := os.Getenv("SERVER_READ_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.ReadTimeout = n
		}
	}
	if v := os.Getenv("SERVER_WRITE_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.WriteTimeout = n
		}
	}
	if v := os.Getenv("SERVER_IDLE_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.IdleTimeout = n
		}
	}

	if v := os.Getenv("AUTH_TYPE"); v != "" {
		cfg.Auth.Type = v
	}

	if v := os.Getenv("KAFKA_BROKERS"); v != "" {
		cfg.Kafka.Brokers = strings.Split(v, ",")
	}
	if v := os.Getenv("KAFKA_SASL_USERNAME"); v != "" {
		cfg.Kafka.SASLUsername = v
	}
	if v := os.Getenv("KAFKA_SASL_PASSWORD"); v != "" {
		cfg.Kafka.SASLPassword = v
	}
	if v := os.Getenv("KAFKA_SASL_MECHANISM"); v != "" {
		cfg.Kafka.SASLMechanism = v
	}
	if v := os.Getenv("KAFKA_SECURITY_PROTOCOL"); v != "" {
		cfg.Kafka.SecurityProtocol = v
	}
	if v := os.Getenv("KAFKA_ACKS"); v != "" {
		cfg.Kafka.Acks = v
	}
	if v := os.Getenv("KAFKA_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Kafka.Retries = n
		}
	}
	if v := os.Getenv("KAFKA_COMPRESSION_TYPE"); v != "" {
		cfg.Kafka.CompressionType = v
	}
}

func validate(cfg *Config) error {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if len(cfg.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka brokers cannot be empty")
	}

	for _, b := range cfg.Kafka.Brokers {
		if strings.Contains(b, "REPLACE_VIA") {
			return fmt.Errorf("kafka broker %q looks like an un-replaced placeholder — set KAFKA_BROKERS", b)
		}
	}

	authType := strings.ToLower(cfg.Auth.Type)
	if authType == "basic" && len(cfg.Auth.Users) == 0 {
		return fmt.Errorf("auth.type is 'basic' but no users are configured")
	}

	if authType == "bearer" && len(cfg.Auth.Tokens) == 0 {
		return fmt.Errorf("auth.type is 'bearer' but no tokens are configured")
	}

	return nil
}

func (c *Config) KafkaConfigMap() map[string]any {
	m := make(map[string]any)

	m["bootstrap.servers"] = strings.Join(c.Kafka.Brokers, ",")

	if c.Kafka.SASLUsername != "" && c.Kafka.SASLPassword != "" {
		m["sasl.username"] = c.Kafka.SASLUsername
		m["sasl.password"] = c.Kafka.SASLPassword
		m["sasl.mechanism"] = c.Kafka.SASLMechanism
		m["security.protocol"] = c.Kafka.SecurityProtocol
	}

	m["acks"] = c.Kafka.Acks
	m["retries"] = c.Kafka.Retries
	m["compression.type"] = c.Kafka.CompressionType

	return m
}
