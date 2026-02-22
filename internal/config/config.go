package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Auth   AuthConfig   `mapstructure:"auth"`
	Kafka  KafkaConfig  `mapstructure:"kafka"`
}

type ServerConfig struct {
	Port         int    `mapstructure:"port"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	IdleTimeout  int    `mapstructure:"idle_timeout"`
}

type AuthConfig struct {
	Type   string       `mapstructure:"type"`
	Users  []UserConfig `mapstructure:"users"`
	Tokens []string     `mapstructure:"tokens"`
}

type UserConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type KafkaConfig struct {
	Brokers          []string `mapstructure:"brokers"`
	SASLUsername     string   `mapstructure:"sasl_username"`
	SASLPassword     string   `mapstructure:"sasl_password"`
	SASLMechanism    string   `mapstructure:"sasl_mechanism"`
	SecurityProtocol string   `mapstructure:"security_protocol"`
	Acks             string   `mapstructure:"acks"`
	Retries          int      `mapstructure:"retries"`
	CompressionType  string   `mapstructure:"compression_type"`
}

func Load(configPath string) (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/kahook")
	}

	setDefaults(v)
	setEnvBindings(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 10)
	v.SetDefault("server.write_timeout", 10)
	v.SetDefault("server.idle_timeout", 60)

	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.acks", "all")
	v.SetDefault("kafka.retries", 3)
	v.SetDefault("kafka.compression_type", "snappy")
	v.SetDefault("kafka.sasl_mechanism", "PLAIN")
	v.SetDefault("kafka.security_protocol", "PLAINTEXT")

	v.SetDefault("auth.type", "none")
}

func setEnvBindings(v *viper.Viper) {
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
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

	// auth.type is accepted for backward compatibility but is no longer
	// enforced — authentication is auto-detected based on whether users
	// and/or tokens are configured. We still warn about obvious misconfigs.
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
	config := make(map[string]any)

	config["bootstrap.servers"] = strings.Join(c.Kafka.Brokers, ",")

	if c.Kafka.SASLUsername != "" && c.Kafka.SASLPassword != "" {
		config["sasl.username"] = c.Kafka.SASLUsername
		config["sasl.password"] = c.Kafka.SASLPassword
		config["sasl.mechanism"] = c.Kafka.SASLMechanism
		config["security.protocol"] = c.Kafka.SecurityProtocol
	}

	config["acks"] = c.Kafka.Acks
	config["retries"] = c.Kafka.Retries
	config["compression.type"] = c.Kafka.CompressionType

	return config
}
