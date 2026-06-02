package config

import (
	"os"
	"path/filepath"

	"github.com/compose-spec/compose-go/v2/template"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type DatabaseConfig struct {
	Driver   string `yaml:"driver"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

type MigrationsConfig struct {
	Parser string `yaml:"parser"`
	Dir    string `yaml:"dir"`
}

type Config struct {
	Database   DatabaseConfig   `yaml:"database"`
	Migrations MigrationsConfig `yaml:"migrations"`
}

func LoadConfig(path string) (*Config, error) {
	// 1. Try to locate project root and load .env
	rootDir := findProjectRoot(path)
	if rootDir != "" {
		_ = godotenv.Load(filepath.Join(rootDir, ".env"))
	} else {
		_ = godotenv.Load() // Fallback to local .env
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 2. Expand environment variables
	mapping := func(s string) (string, bool) {
		return os.LookupEnv(s)
	}

	expandedData, err := template.Substitute(string(data), mapping)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal([]byte(expandedData), &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func findProjectRoot(configPath string) string {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return ""
	}
	dir := filepath.Dir(absPath)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
