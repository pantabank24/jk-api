package repository

import (
	"math"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

type BalanceSummary struct {
	Balance     float64 `json:"balance"`
	TotalWeight float64 `json:"total_weight"`
	AvgPrice    float64 `json:"avg_price"`
}

type BillBalanceRepository interface {
	Record(userID uint, storeID *uint, quotationID *uint, amount float64, weight float64, avgPrice float64, description string) error
	GetBalance(userID uint) (BalanceSummary, error)
	GetHistory(userID uint, limit int) ([]entity.BillBalance, error)
}

type billBalanceRepository struct {
	db *gorm.DB
}

func NewBillBalanceRepository(db *gorm.DB) BillBalanceRepository {
	return &billBalanceRepository{db: db}
}

func (r *billBalanceRepository) Record(userID uint, storeID *uint, quotationID *uint, amount float64, weight float64, avgPrice float64, description string) error {
	return r.db.Create(&entity.BillBalance{
		UserID:      userID,
		StoreID:     storeID,
		QuotationID: quotationID,
		Amount:      amount,
		Weight:      weight,
		AvgPrice:    avgPrice,
		Description: description,
	}).Error
}

// GetBalance walks all live records in chronological order and accumulates
// weight/avgPrice only within the current unsettled cycle. When the running
// balance reaches zero (settled), the weight/price accumulators reset so past
// cycles have no effect. Rows settled by เคลียร์บิล (settled_at) are excluded
// entirely — they remain only as history.
func (r *billBalanceRepository) GetBalance(userID uint) (BalanceSummary, error) {
	var records []entity.BillBalance
	err := r.db.Where("user_id = ? AND settled_at IS NULL", userID).
		Order("created_at ASC").
		Find(&records).Error
	if err != nil {
		return BalanceSummary{}, err
	}

	var balance, weightSum, priceWeightSum float64
	for _, rec := range records {
		balance += rec.Amount
		weightSum += rec.Weight
		priceWeightSum += rec.Weight * rec.AvgPrice
		// Round to satang to avoid float drift triggering a false reset.
		if math.Round(balance*100) == 0 {
			balance = 0
			weightSum = 0
			priceWeightSum = 0
		}
	}

	var avgPrice float64
	if weightSum > 0 {
		avgPrice = priceWeightSum / weightSum
	}

	return BalanceSummary{
		Balance:     balance,
		TotalWeight: weightSum,
		AvgPrice:    avgPrice,
	}, nil
}

func (r *billBalanceRepository) GetHistory(userID uint, limit int) ([]entity.BillBalance, error) {
	var records []entity.BillBalance
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}
