package config

import (
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	Env      string     `yaml:"env" env:"ENV" env-default:"local"`
	Telegram Telegram   `yaml:"telegram"`
	Database Database   `yaml:"database"`
	LLM      LLMConfig  `yaml:"llm"`
	Server   HTTPServer `yaml:"server"`
}

type Telegram struct {
	Token string `yaml:"token" env-required:"true" env:"TELEGRAM_TOKEN"`
}

type Database struct {
	Host        string `yaml:"host" env:"DB_HOST" env-default:"localhost"`
	Port        int    `yaml:"port" env:"DB_PORT" env-default:"5432"`
	User        string `yaml:"user" env:"DB_USER" env-required:"true"`
	Password    string `yaml:"password" env:"DB_PASSWORD" env-required:"true"`
	Name        string `yaml:"name" env:"DB_NAME" env-required:"true"`
	SSLMode     string `yaml:"sslmode" env:"DB_SSLMODE" env-default:"disable"`
	DatabaseUrl string
}

type LLMConfig struct {
	Provider string        `yaml:"provider" env:"LLM_PROVIDER" env-default:"openai"`
	Model    string        `yaml:"model" env:"LLM_MODEL" env-default:"gpt-3.5-turbo"`
	Timeout  time.Duration `yaml:"timeout" env:"LLM_TIMEOUT" env-default:"30s"`
	APIKey   string        `yaml:"api_key" env-required:"true" env:"LLM_API_KEY"`
}

type HTTPServer struct {
	Address     string        `yaml:"address" env:"SERVER_ADDRESS" env-default:":8080"`
	Timeout     time.Duration `yaml:"timeout" env:"SERVER_TIMEOUT" env-default:"4s"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env:"SERVER_IDLE_TIMEOUT" env-default:"120s"`
}

func MustLoad() *Config {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		log.Fatal("CONFIG_PATH is not set")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("Config file %s does not found", configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("Failed to load config: %s", err)
	}

	dbURL := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(cfg.Database.Host, strconv.Itoa(cfg.Database.Port)),
		Path:   cfg.Database.Name,
		User:   url.UserPassword(cfg.Database.User, cfg.Database.Password),
	}

	query := dbURL.Query()
	query.Set("sslmode", cfg.Database.SSLMode)
	dbURL.RawQuery = query.Encode()

	cfg.Database.DatabaseUrl = dbURL.String()

	return &cfg
}
