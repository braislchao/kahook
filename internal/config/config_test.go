package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Run from a temp dir so no config.yaml is found.
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("Default port should be 8080, got %d", cfg.Server.Port)
	}

	if cfg.Kafka.Acks != "all" {
		t.Errorf("Default acks should be 'all', got %s", cfg.Kafka.Acks)
	}

	if cfg.Kafka.Retries != 3 {
		t.Errorf("Default retries should be 3, got %d", cfg.Kafka.Retries)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	t.Setenv("SERVER_PORT", "9999")
	t.Setenv("KAFKA_RETRIES", "10")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 9999 {
		t.Errorf("Port should be 9999 from env, got %d", cfg.Server.Port)
	}

	if cfg.Kafka.Retries != 10 {
		t.Errorf("Retries should be 10 from env, got %d", cfg.Kafka.Retries)
	}
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	data := []byte(`
server:
  port: 3000
kafka:
  brokers:
    - broker1:9092
    - broker2:9092
  acks: "1"
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 3000 {
		t.Errorf("Port should be 3000 from file, got %d", cfg.Server.Port)
	}

	if len(cfg.Kafka.Brokers) != 2 {
		t.Errorf("Should have 2 brokers, got %d", len(cfg.Kafka.Brokers))
	}

	if cfg.Kafka.Acks != "1" {
		t.Errorf("Acks should be '1' from file, got %s", cfg.Kafka.Acks)
	}

	// Defaults still apply for fields not in the file.
	if cfg.Kafka.CompressionType != "snappy" {
		t.Errorf("Default compression_type should be 'snappy', got %s", cfg.Kafka.CompressionType)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	data := []byte(`
server:
  port: 3000
kafka:
  brokers:
    - file-broker:9092
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SERVER_PORT", "5555")
	t.Setenv("KAFKA_BROKERS", "env-broker:9092")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 5555 {
		t.Errorf("Env should override file port, got %d", cfg.Server.Port)
	}

	if len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Brokers[0] != "env-broker:9092" {
		t.Errorf("Env should override file brokers, got %v", cfg.Kafka.Brokers)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 0},
		Kafka:  KafkaConfig{Brokers: []string{"localhost:9092"}},
		Auth:   AuthConfig{Type: "none"},
	}

	if err := validate(cfg); err == nil {
		t.Error("Should fail with invalid port")
	}

	cfg.Server.Port = 70000
	if err := validate(cfg); err == nil {
		t.Error("Should fail with port > 65535")
	}
}

func TestValidate_EmptyBrokers(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Kafka:  KafkaConfig{Brokers: []string{}},
		Auth:   AuthConfig{Type: "none"},
	}

	if err := validate(cfg); err == nil {
		t.Error("Should fail with empty brokers")
	}
}

func TestValidate_PlaceholderBroker(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Kafka:  KafkaConfig{Brokers: []string{"REPLACE_VIA_ENV_KAFKA_BROKERS"}},
		Auth:   AuthConfig{Type: "none"},
	}

	err := validate(cfg)
	if err == nil {
		t.Error("Should fail with placeholder broker")
	}
	if err != nil && !strings.Contains(err.Error(), "REPLACE_VIA") {
		t.Errorf("Error should mention REPLACE_VIA, got: %v", err)
	}
}

func TestValidate_AnyAuthTypeAccepted(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Kafka:  KafkaConfig{Brokers: []string{"localhost:9092"}},
		Auth:   AuthConfig{Type: "custom"},
	}

	if err := validate(cfg); err != nil {
		t.Errorf("Should accept any auth.type value, got error: %v", err)
	}
}

func TestValidate_BasicAuthNoUsers(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Kafka:  KafkaConfig{Brokers: []string{"localhost:9092"}},
		Auth:   AuthConfig{Type: "basic", Users: []UserConfig{}},
	}

	if err := validate(cfg); err == nil {
		t.Error("Should fail with basic auth and no users")
	}
}

func TestValidate_BearerAuthNoTokens(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Kafka:  KafkaConfig{Brokers: []string{"localhost:9092"}},
		Auth:   AuthConfig{Type: "bearer", Tokens: []string{}},
	}

	if err := validate(cfg); err == nil {
		t.Error("Should fail with bearer auth and no tokens")
	}
}

func TestValidate_ValidBasicAuth(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Kafka:  KafkaConfig{Brokers: []string{"localhost:9092"}},
		Auth: AuthConfig{
			Type:  "basic",
			Users: []UserConfig{{Username: "admin", Password: "secret"}},
		},
	}

	if err := validate(cfg); err != nil {
		t.Errorf("Should pass with valid basic auth: %v", err)
	}
}

func TestValidate_ValidBearerAuth(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 8080},
		Kafka:  KafkaConfig{Brokers: []string{"localhost:9092"}},
		Auth: AuthConfig{
			Type:   "bearer",
			Tokens: []string{"token123"},
		},
	}

	if err := validate(cfg); err != nil {
		t.Errorf("Should pass with valid bearer auth: %v", err)
	}
}

func TestLoad_AuthTokensFromEnv(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	t.Setenv("AUTH_TYPE", "bearer")
	t.Setenv("AUTH_TOKENS", "tok1,tok2,tok3")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Auth.Tokens) != 3 {
		t.Fatalf("Expected 3 tokens, got %d", len(cfg.Auth.Tokens))
	}
	if cfg.Auth.Tokens[0] != "tok1" || cfg.Auth.Tokens[1] != "tok2" || cfg.Auth.Tokens[2] != "tok3" {
		t.Errorf("Unexpected tokens: %v", cfg.Auth.Tokens)
	}
}

func TestLoad_AuthBasicUsersFromEnv(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	t.Setenv("AUTH_TYPE", "basic")
	t.Setenv("AUTH_BASIC_USERS", "admin:secret,reader:pass123")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Auth.Users) != 2 {
		t.Fatalf("Expected 2 users, got %d", len(cfg.Auth.Users))
	}
	if cfg.Auth.Users[0].Username != "admin" || cfg.Auth.Users[0].Password != "secret" {
		t.Errorf("Unexpected first user: %+v", cfg.Auth.Users[0])
	}
	if cfg.Auth.Users[1].Username != "reader" || cfg.Auth.Users[1].Password != "pass123" {
		t.Errorf("Unexpected second user: %+v", cfg.Auth.Users[1])
	}
}

func TestLoad_AuthBasicUsersFromEnv_MalformedSkipped(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	t.Setenv("AUTH_TYPE", "basic")
	t.Setenv("AUTH_BASIC_USERS", "admin:secret,badentry,reader:pass")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Auth.Users) != 2 {
		t.Fatalf("Expected 2 valid users (malformed skipped), got %d", len(cfg.Auth.Users))
	}
	if cfg.Auth.Users[0].Username != "admin" || cfg.Auth.Users[1].Username != "reader" {
		t.Errorf("Unexpected users: %+v", cfg.Auth.Users)
	}
}

func TestLoad_AuthEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	data := []byte(`
auth:
  type: bearer
  tokens:
    - file-token
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AUTH_TOKENS", "env-token1,env-token2")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Auth.Tokens) != 2 || cfg.Auth.Tokens[0] != "env-token1" {
		t.Errorf("Env should override file tokens, got %v", cfg.Auth.Tokens)
	}
}

func TestLoad_AuthBasicUsersPasswordWithColon(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	t.Setenv("AUTH_TYPE", "basic")
	t.Setenv("AUTH_BASIC_USERS", "admin:pass:with:colons")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Auth.Users) != 1 {
		t.Fatalf("Expected 1 user, got %d", len(cfg.Auth.Users))
	}
	if cfg.Auth.Users[0].Password != "pass:with:colons" {
		t.Errorf("Password should preserve colons, got %q", cfg.Auth.Users[0].Password)
	}
}

func TestKafkaConfigMap(t *testing.T) {
	tests := []struct {
		name   string
		kafka  KafkaConfig
		checks map[string]any
	}{
		{
			name: "basic config",
			kafka: KafkaConfig{
				Brokers:         []string{"localhost:9092"},
				Acks:            "all",
				Retries:         3,
				CompressionType: "snappy",
			},
			checks: map[string]any{
				"bootstrap.servers": "localhost:9092",
				"acks":              "all",
			},
		},
		{
			name: "with SASL",
			kafka: KafkaConfig{
				Brokers:          []string{"broker:9092"},
				SASLUsername:     "user",
				SASLPassword:     "pass",
				SASLMechanism:    "PLAIN",
				SecurityProtocol: "SASL_SSL",
			},
			checks: map[string]any{
				"sasl.username":     "user",
				"sasl.password":     "pass",
				"sasl.mechanism":    "PLAIN",
				"security.protocol": "SASL_SSL",
			},
		},
		{
			name: "multiple brokers",
			kafka: KafkaConfig{
				Brokers: []string{"broker1:9092", "broker2:9092", "broker3:9092"},
			},
			checks: map[string]any{
				"bootstrap.servers": "broker1:9092,broker2:9092,broker3:9092",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Kafka: tt.kafka}
			configMap := cfg.KafkaConfigMap()

			for key, expected := range tt.checks {
				if got := configMap[key]; got != expected {
					t.Errorf("KafkaConfigMap[%s] = %v, want %v", key, got, expected)
				}
			}
		})
	}
}
