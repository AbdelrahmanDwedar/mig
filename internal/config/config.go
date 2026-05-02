package config

import (
	"os"

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
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
