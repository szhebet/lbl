package config

import (
	"fmt"
	"os"
	"path/filepath"

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

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

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
