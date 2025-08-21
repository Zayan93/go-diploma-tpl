package config

import (
	"flag"
	"os"
)

type Config struct {
	Address              string // адрес запуска HTTP-сервера, например localhost:8080
	BaseURL              string // базовый URL для сокращённых ссылок, например http://localhost:8080
	LogLevel             string // Уровень логирования
	FileStoragePath      string // путь до файла с данными
	DatabaseDSN          string // подключение к базе данных (host)
	AccrualSystemAddress string // адрес системы расчёта начислений
	JWTSecret            string // секретный ключ для JWT токенов
}

// New создает и инициализирует конфигурацию из флагов командной строки
func New() *Config {
	defaultAddress := "localhost:8080"
	defaultBaseURL := "http://localhost:8080"
	defaultLogLevel := "info"
	defaultFileStoragePath := "./storage.txt"
	defaultDBDSN := "host=localhost user=postgres password=fmx274TQVw111w111w dbname=users sslmode=disable"
	defaultAccrualSystemAddress := "http://localhost:8081"
	defaultJWTSecret := "your-secret-key-here"

	// Приоритет: переменные окружения, затем флаги командной строки
	envAddress := os.Getenv("RUN_ADDRESS")
	envBaseURL := os.Getenv("BASE_URL")
	envLogLevel := os.Getenv("LOG_LEVEL")
	envFileStoragePath := os.Getenv("FILE_STORAGE_PATH")
	envDatabaseDSN := os.Getenv("DATABASE_URI")
	envAccrualSystemAddress := os.Getenv("ACCRUAL_SYSTEM_ADDRESS")
	envJWTSecret := os.Getenv("JWT_SECRET")

	// Устанавливаем значения по умолчанию, если переменные окружения не заданы
	if envAddress == "" {
		envAddress = defaultAddress
	}
	if envBaseURL == "" {
		envBaseURL = defaultBaseURL
	}
	if envLogLevel == "" {
		envLogLevel = defaultLogLevel
	}
	if envFileStoragePath == "" {
		envFileStoragePath = defaultFileStoragePath
	}
	if envDatabaseDSN == "" {
		envDatabaseDSN = defaultDBDSN
	}
	if envAccrualSystemAddress == "" {
		envAccrualSystemAddress = defaultAccrualSystemAddress
	}
	if envJWTSecret == "" {
		envJWTSecret = defaultJWTSecret
	}

	// Флаги командной строки (имеют приоритет над переменными окружения)
	addr := flag.String("a", envAddress, "HTTP server address")
	baseURL := flag.String("b", envBaseURL, "Base URL for short links")
	logLevel := flag.String("l", envLogLevel, "Log level")
	fileStoragePath := flag.String("f", envFileStoragePath, "File storage path")
	databaseDSN := flag.String("d", envDatabaseDSN, "Database DSN for connection")
	accrualSystemAddress := flag.String("r", envAccrualSystemAddress, "Accrual system address")
	jwtSecret := flag.String("j", envJWTSecret, "JWT secret key")
	flag.Parse()

	return &Config{
		Address:              *addr,
		BaseURL:              *baseURL,
		LogLevel:             *logLevel,
		FileStoragePath:      *fileStoragePath,
		DatabaseDSN:          *databaseDSN,
		AccrualSystemAddress: *accrualSystemAddress,
		JWTSecret:            *jwtSecret,
	}
}
