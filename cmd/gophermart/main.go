package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/Zayan93/go-diploma-tpl/internal/app"
	"github.com/Zayan93/go-diploma-tpl/internal/compressor"
	"github.com/Zayan93/go-diploma-tpl/internal/config"
	"github.com/Zayan93/go-diploma-tpl/internal/logger"
	"github.com/Zayan93/go-diploma-tpl/internal/middleware"
	"github.com/Zayan93/go-diploma-tpl/internal/services"
	"github.com/Zayan93/go-diploma-tpl/internal/store"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

func main() {

	cfg := config.New()
	flagLogLevel := cfg.LogLevel

	if err := logger.Initialize(flagLogLevel); err != nil {
		// Используем стандартный log на случай ошибки при ините нашего лога
		log.Fatalf("failed to initialize logger: %v", err)
	}

	defer logger.Log.Sync()

	var userStorage store.UserStorage

	// Попытка PostgreSQL
	if cfg.DatabaseDSN != "" {
		db, err := sql.Open("pgx", cfg.DatabaseDSN)
		if err != nil {
			logger.Log.Error("Failed to open database connection", zap.Error(err))
		} else {
			ctx := context.Background()
			if err = db.PingContext(ctx); err != nil {
				logger.Log.Error("Failed to ping database", zap.Error(err))
			} else {
				psqlStorage, err := store.NewSQLStorage(db)
				if err != nil {
					logger.Log.Error("Failed to initialize SQL storage", zap.Error(err))
				} else {
					logger.Log.Info("Connected to PSQL server")
					userStorage = psqlStorage // SQLStorage реализует оба интерфейса
				}
			}
		}
	}

	if userStorage == nil {
		log.Fatalf("failed to initialize any storage backend")
	}

	// Создаем сервис аутентификации
	authService := services.NewAuthService(cfg.JWTSecret)

	// Baseurl передаю через dependency injection в хендлеры
	handler := app.NewHandler(userStorage, authService, cfg)

	r := chi.NewRouter()
	r.Use(compressor.GzipMiddleware)
	r.Use(logger.WithLogging)

	// Маршруты для аутентификации (без middleware)
	r.Post("/api/user/register", handler.PostRegister)
	r.Post("/api/user/login", handler.PostLogin)

	// Маршруты для заказов (с middleware аутентификации)
	r.With(middleware.AuthMiddleware(authService)).Post("/api/user/orders", handler.PostOrders)
	r.With(middleware.AuthMiddleware(authService)).Get("/api/user/orders", handler.GetOrders)

	// Маршрут для получения баланса пользователя (с middleware аутентификации)
	r.With(middleware.AuthMiddleware(authService)).Get("/api/user/balance", handler.GetUserBalance)

	// Маршруты для работы с выводами средств (с middleware аутентификации)
	r.With(middleware.AuthMiddleware(authService)).Post("/api/user/balance/withdraw", handler.PostWithdrawBalance)
	r.With(middleware.AuthMiddleware(authService)).Get("/api/user/withdrawals", handler.GetUserWithdrawals)

	logger.Log.Info("Running server", zap.String("address", cfg.Address))
	logger.Log.Info("Accrual system address", zap.String("address", cfg.AccrualSystemAddress))

	// Используем стандартный log тк пишет сразу в stderr и завершает программу
	log.Fatal(http.ListenAndServe(cfg.Address, r))

}
