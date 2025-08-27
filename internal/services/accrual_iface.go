package services

import (
	"context"

	"github.com/Zayan93/go-diploma-tpl/internal/models"
)

type AccrualServiceIface interface {
	GetOrderInfo(ctx context.Context, orderNumber string) (*models.AccrualResponse, error)
}
