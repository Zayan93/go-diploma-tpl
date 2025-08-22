package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

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
	var db *sql.DB

	// Попытка PostgreSQL
	if cfg.DatabaseDSN != "" {
		var err error
		db, err = sql.Open("pgx", cfg.DatabaseDSN)
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

	// Закрываем соединение с базой данных при завершении
	defer func() {
		if db != nil {
			if err := db.Close(); err != nil {
				logger.Log.Error("Failed to close database connection", zap.Error(err))
			} else {
				logger.Log.Info("Database connection closed")
			}
		}
	}()

	// Создаем сервис аутентификации
	authService := services.NewAuthService(cfg.JWTSecret)

	// Создаем сервис начисления баллов
	accrualService := services.NewAccrualService(cfg.AccrualSystemAddress)

	// Baseurl передаю через dependency injection в хендлеры
	handler := app.NewHandler(userStorage, authService, accrualService, cfg)

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

	// Запускаем фоновую обработку заказов
	go func() {
		logger.Log.Info("Starting background order processing")
		for {
			processOrders(userStorage.(store.OrderStorage), accrualService)
			time.Sleep(10 * time.Second) // Обрабатываем заказы каждые 10 секунд
		}
	}()

	logger.Log.Info("Running server", zap.String("address", cfg.Address))
	logger.Log.Info("Accrual system address", zap.String("address", cfg.AccrualSystemAddress))

	// Используем стандартный log тк пишет сразу в stderr и завершает программу
	log.Fatal(http.ListenAndServe(cfg.Address, r))

}

// processOrders обрабатывает заказы со статусом NEW и PROCESSING
func processOrders(storage store.OrderStorage, accrualService *services.AccrualService) {
	ctx := context.Background()

	// Получаем заказы для обработки
	orders, err := storage.GetOrdersByStatus(ctx, []string{"NEW", "PROCESSING"})
	if err != nil {
		logger.Log.Error("Failed to get orders for processing", zap.Error(err))
		return
	}

	for _, order := range orders {
		// Обновляем статус на PROCESSING если он NEW
		if order.Status == "NEW" {
			err := storage.UpdateOrderStatus(ctx, order.ID, "PROCESSING")
			if err != nil {
				logger.Log.Error("Failed to update order status to PROCESSING",
					zap.String("orderNum", order.OrderNum), zap.Error(err))
				continue
			}
			logger.Log.Info("Order status updated to PROCESSING",
				zap.String("orderNum", order.OrderNum))
		}

		// Получаем информацию о заказе из системы начисления
		accrualInfo, err := accrualService.GetOrderInfo(ctx, order.OrderNum)
		if err != nil {
			logger.Log.Error("Failed to get accrual info",
				zap.String("orderNum", order.OrderNum), zap.Error(err))
			continue
		}

		if accrualInfo == nil {
			// Заказ не найден в системе начисления, пропускаем
			continue
		}

		// Обрабатываем результат в зависимости от статуса
		switch accrualInfo.Status {
		case "PROCESSED":
			if accrualInfo.Accrual != nil {
				// Обновляем заказ и баланс пользователя
				err = storage.UpdateOrderStatusAndBalance(ctx, order.OrderNum, "PROCESSED",
					accrualInfo.Accrual, order.UserID, *accrualInfo.Accrual, 0)
				if err != nil {
					logger.Log.Error("Failed to update order and balance",
						zap.String("orderNum", order.OrderNum), zap.Error(err))
				} else {
					logger.Log.Info("Order processed successfully",
						zap.String("orderNum", order.OrderNum),
						zap.Float64("accrual", *accrualInfo.Accrual))
				}
			}
		case "INVALID":
			// Заказ не принят к расчёту
			err = storage.UpdateOrderStatus(ctx, order.ID, "INVALID")
			if err != nil {
				logger.Log.Error("Failed to update order status to INVALID",
					zap.String("orderNum", order.OrderNum), zap.Error(err))
			} else {
				logger.Log.Info("Order marked as INVALID",
					zap.String("orderNum", order.OrderNum))
			}
		case "PROCESSING":
			// Заказ в процессе обработки, оставляем как есть
			logger.Log.Debug("Order still processing",
				zap.String("orderNum", order.OrderNum))
		default:
			logger.Log.Warn("Unknown accrual status",
				zap.String("orderNum", order.OrderNum),
				zap.String("status", accrualInfo.Status))
		}
	}
}
