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
}

type ServerConfig struct {
	Port         int    `toml:"port"`
	Bind         string `toml:"bind"`
	EnableDelete bool   `toml:"enable_delete"`
	LogLevel     string `toml:"log_level"`
	JWTSecret    string `toml:"jwt_secret"`
	TokenTTL     int    `toml:"token_ttl"` // hours; 0 = no expiration
}

type DirectoriesConfig struct {
	Bookarch  string `toml:"bookarch"`
	Temp      string `toml:"temp"`
	Logs      string `toml:"logs"`
	Templates string `toml:"templates"`
	Static    string `toml:"static"`
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
	BaseURL string `toml:"base_url"`
	Model   string `toml:"model"`
	Token   string `toml:"token"`
	Prompt  string `toml:"prompt"`
	Prompt2 string `toml:"prompt2"`
	Timeout int    `toml:"timeout"`
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
			Password: "postgres",
			SSLMode:  "disable",
			DataDir:  "/var/lib/postgresql/data",
		},
		LLM: LLMConfig{
			BaseURL: "http://localhost:8080",
			Model:   "llama",
			Token:   "",
			Prompt:  "в тексте первых 3х страниц книги найди ФИО автора и название произведения, в результате верни только 2 строки в формате AUTHOR:ФИО автора BOOKNAME: название произведения",
			Prompt2: "по цитате нескольких страниц определи ФИО автора и название произведения, в результате верни только 2 строки в формате AUTHOR:ФИО автора BOOKNAME: название произведения",
			Timeout: 60,
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
