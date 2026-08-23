package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server     ServerConfig     `toml:"server"`
	Directories DirectoriesConfig `toml:"directories"`
	Database   DatabaseConfig   `toml:"database"`
	LLM        LLMConfig        `toml:"llm"`
	Federation FederationConfig `toml:"federation"`
}

type ServerConfig struct {
	Port         int    `toml:"port"`
	Bind         string `toml:"bind"`
	EnableDelete bool   `toml:"enable_delete"`
	LogLevel     string `toml:"log_level"`
	JWTSecret    string `toml:"jwt_secret"`
	TokenTTL     int    `toml:"token_ttl"` // hours; 0 = no expiration
	PublicURL    string `toml:"public_url"`
}

type DirectoriesConfig struct {
	Bookarch  string `toml:"bookarch"`
	Temp      string `toml:"temp"`
	Logs      string `toml:"logs"`
	Templates string `toml:"templates"`
	Static    string `toml:"static"`
	Backup    string `toml:"backup"`
	ApkDir    string `toml:"apk_dir"`
}

type DatabaseConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Name     string `toml:"name"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	SSLMode  string `toml:"sslmode"`
	DataDir  string `toml:"pgdata"`
}

type LLMConfig struct {
	BaseURL      string `toml:"base_url"`
	Model        string `toml:"model"`
	Token        string `toml:"token"`
	Prompt       string `toml:"prompt"`
	Prompt2      string `toml:"prompt2"`
	PromptConvert string `toml:"prompt_convert"`
	Timeout      int    `toml:"timeout"`
}

// FederationConfig controls the background distribution of user book requests
// to the registered peer library servers (api_neighbours).
type FederationConfig struct {
	// Enabled turns the background distributor on/off.
	Enabled bool `toml:"enabled"`
	// PushIntervalSec is how often the background loop wakes to deliver requests.
	PushIntervalSec int `toml:"push_interval_sec"`
	// RetryIntervalSec is the time between delivery attempts to an unreachable
	// neighbour.
	RetryIntervalSec int `toml:"retry_interval_sec"`
	// RetryWindowSec is how long an undelivered request is retried before the
	// distributor gives up on that neighbour for it.
	RetryWindowSec int `toml:"retry_window_sec"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 9091,
			Bind: "0.0.0.0",
			LogLevel: "info",
		},
		Directories: DirectoriesConfig{
			Bookarch:  "bookarch",
			Temp:      "tempfld",
			Logs:      "logs",
			Templates: "templates",
			Static:    "static",
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Name:     "library",
			User:     "postgres",
			SSLMode:  "disable",
			DataDir:  "/var/lib/postgresql/data",
		},
		LLM: LLMConfig{
			BaseURL:      "http://localhost:8080",
			Model:        "llama",
			Token:        "",
			Prompt:       "в тексте первых 3х страниц книги найди ФИО автора и название произведения, в результате верни только 2 строки в формате AUTHOR:ФИО автора BOOKNAME: название произведения",
			Prompt2:      "по цитате нескольких страниц определи ФИО автора и название произведения, в результате верни только 2 строки в формате AUTHOR:ФИО автора BOOKNAME: название произведения",
			PromptConvert: "Преобразуй к формату Автор - Название произведения следующий текст: \n",
			Timeout:      60,
		},
		Federation: FederationConfig{
			Enabled:          true,
			PushIntervalSec:  300,
			RetryIntervalSec: 60,
			RetryWindowSec:   3600,
		},
	}
}

func (c *Config) DSNDisplay() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s dbname=%s sslmode=%s",
		c.Database.Host, c.Database.Port, c.Database.User, c.Database.Name, c.Database.SSLMode,
	)
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host, c.Database.Port, c.Database.User,
		c.Database.Password, c.Database.Name, c.Database.SSLMode,
	)
}

func (c *Config) ensureDirs() error {
	dirs := []string{
		c.Directories.Bookarch,
		filepath.Join(c.Directories.Bookarch, "covers"),
		c.Directories.Temp,
		c.Directories.Logs,
	}

	tmpBookarch := filepath.Join(filepath.Dir(c.Directories.Bookarch), "tmpBookarch")
	dirs = append(dirs, tmpBookarch)

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("LIBAPP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	} else if v := os.Getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("LIBAPP_BIND"); v != "" {
		cfg.Server.Bind = v
	}
	if v := os.Getenv("LIBAPP_ENABLE_DELETE"); v != "" {
		cfg.Server.EnableDelete = v == "true" || v == "1" || v == "yes"
	}
	if v := os.Getenv("LIBAPP_LOG_LEVEL"); v != "" {
		cfg.Server.LogLevel = v
	}
	if v := os.Getenv("LIBAPP_JWT_SECRET"); v != "" {
		cfg.Server.JWTSecret = v
	}
	if v := os.Getenv("LIBAPP_TOKEN_TTL"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.TokenTTL = p
		}
	}
	if v := os.Getenv("LIBAPP_PUBLIC_URL"); v != "" {
		cfg.Server.PublicURL = v
	}

	if v := os.Getenv("LIBAPP_DIR_BOOKARCH"); v != "" {
		cfg.Directories.Bookarch = v
	}
	if v := os.Getenv("LIBAPP_DIR_TEMP"); v != "" {
		cfg.Directories.Temp = v
	}
	if v := os.Getenv("LIBAPP_DIR_LOGS"); v != "" {
		cfg.Directories.Logs = v
	}
	if v := os.Getenv("LIBAPP_DIR_TEMPLATES"); v != "" {
		cfg.Directories.Templates = v
	}
	if v := os.Getenv("LIBAPP_DIR_STATIC"); v != "" {
		cfg.Directories.Static = v
	}
	if v := os.Getenv("LIBAPP_DIR_BACKUP"); v != "" {
		cfg.Directories.Backup = v
	}
	if v := os.Getenv("LIBAPP_DIR_APK"); v != "" {
		cfg.Directories.ApkDir = v
	}

	dsn := os.Getenv("LIBAPP_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn != "" {
		applyDSN(cfg, dsn)
	}
	if v := os.Getenv("LIBAPP_DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("LIBAPP_DB_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Database.Port = p
		}
	}
	if v := os.Getenv("LIBAPP_DB_NAME"); v != "" {
		cfg.Database.Name = v
	}
	if v := os.Getenv("LIBAPP_DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("LIBAPP_DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("LIBAPP_DB_SSLMODE"); v != "" {
		cfg.Database.SSLMode = v
	}
	if v := os.Getenv("LIBAPP_DB_PGDATA"); v != "" {
		cfg.Database.DataDir = v
	}

	if v := os.Getenv("LIBAPP_LLM_BASE_URL"); v != "" {
		cfg.LLM.BaseURL = v
	}
	if v := os.Getenv("LIBAPP_LLM_MODEL"); v != "" {
		cfg.LLM.Model = v
	}
	if v := os.Getenv("LIBAPP_LLM_TOKEN"); v != "" {
		cfg.LLM.Token = v
	}
	if v := os.Getenv("LIBAPP_LLM_TIMEOUT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.LLM.Timeout = p
		}
	}
	if v := os.Getenv("LIBAPP_LLM_PROMPT"); v != "" {
		cfg.LLM.Prompt = v
	}
	if v := os.Getenv("LIBAPP_LLM_PROMPT2"); v != "" {
		cfg.LLM.Prompt2 = v
	}
	if v := os.Getenv("LIBAPP_LLM_PROMPT_CONVERT"); v != "" {
		cfg.LLM.PromptConvert = v
	}

	if v := os.Getenv("LIBAPP_FED_ENABLED"); v != "" {
		cfg.Federation.Enabled = v == "true" || v == "1" || v == "yes"
	}
	if v := os.Getenv("LIBAPP_FED_PUSH_INTERVAL_SEC"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Federation.PushIntervalSec = p
		}
	}
	if v := os.Getenv("LIBAPP_FED_RETRY_INTERVAL_SEC"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Federation.RetryIntervalSec = p
		}
	}
	if v := os.Getenv("LIBAPP_FED_RETRY_WINDOW_SEC"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Federation.RetryWindowSec = p
		}
	}
}

func applyDSN(cfg *Config, dsn string) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return
		}
		cfg.Database.Host = u.Hostname()
		if p := u.Port(); p != "" {
			if port, err := strconv.Atoi(p); err == nil {
				cfg.Database.Port = port
			}
		}
		if u.User != nil {
			cfg.Database.User = u.User.Username()
			cfg.Database.Password, _ = u.User.Password()
		}
		cfg.Database.Name = strings.TrimPrefix(u.Path, "/")
		if sslmode := u.Query().Get("sslmode"); sslmode != "" {
			cfg.Database.SSLMode = sslmode
		}
		return
	}
	for _, part := range strings.Fields(dsn) {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "host":
			cfg.Database.Host = kv[1]
		case "port":
			if port, err := strconv.Atoi(kv[1]); err == nil {
				cfg.Database.Port = port
			}
		case "user":
			cfg.Database.User = kv[1]
		case "password":
			cfg.Database.Password = kv[1]
		case "dbname":
			cfg.Database.Name = kv[1]
		case "sslmode":
			cfg.Database.SSLMode = kv[1]
		}
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// 1. Apply env vars (lower priority, config.toml overrides them)
	applyEnv(cfg)

	if path == "" {
		path = os.Getenv("CONFIG_PATH")
	}
	if path == "" {
		path = "config.toml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := cfg.ensureDirs(); err != nil {
				return nil, err
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	// 2. Apply config.toml (higher priority, overrides env vars)
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	if err := cfg.ensureDirs(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) TempBookarchDir() string {
	return filepath.Join(filepath.Dir(c.Directories.Bookarch), "tmpBookarch")
}
