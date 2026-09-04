package repository

import (
	"time"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// SellOrderFilter is the shared "which orders" question behind the customer's own
// list and the master's system-wide list.
type SellOrderFilter struct {
	// UserID scopes to one customer. Always set for the customer's own list.
	UserID *uint
	// Status narrows to one lifecycle state; empty = every state.
	Status string
	// Search matches the customer's name/email (master list only).
	Search string
}

// ActiveTotals is a customer's outstanding exposure, checked against the
// auto_sell_max_active_* caps before a new order is accepted.
type ActiveTotals struct {
	Count  int64   `json:"count"`
	Weight float64 `json:"weight"`
}

type SellOrderRepository interface {
	Create(o *entity.SellOrder) error
	FindAll(f SellOrderFilter, page, limit int) ([]entity.SellOrder, int64, error)
	FindByID(id uint) (*entity.SellOrder, error)
	// ActiveTotalsByUser sums the customer's waiting orders (count + weight) so the
	// caps can be enforced on create.
	ActiveTotalsByUser(userID uint) (ActiveTotals, error)
	Cancel(id uint, reason string) error
	// ClaimDue atomically flips every active order whose target the price has
	// reached to 'filling' and returns them. Claiming and selecting in one
	// statement is what stops two ticks (or a slow tick overlapping the next)
	// from filling the same order twice.
	ClaimDue(price float64) ([]entity.SellOrder, error)
	// MarkFilled completes a claimed order and links the bill it produced.
	MarkFilled(id uint, price float64, billID uint) error
	// RecordFillPrice stamps the price a claimed order is filling at, before the
	// bill is written. A fill's items join whatever bill the customer has open, so
	// that bill's total can no longer be divided back into a unit price — this is
	// what the boot recovery reads to complete an interrupted fill.
	RecordFillPrice(id uint, price float64) error
	// ReleaseClaim puts a claimed order back on the market — used when the fill
	// itself failed, so the next tick can retry instead of losing the order.
	ReleaseClaim(id uint) error
	// FindStuckFilling returns orders left mid-fill by a crash or restart.
	FindStuckFilling() ([]entity.SellOrder, error)
	// FindBillBySellOrder reports the bill an order's items landed in, if any — its
	// own new bill, or the customer's open one they were appended to. The boot
	// recovery uses it to tell "the fill completed but we died before marking it"
	// apart from "the fill never happened", so it is answered from the items, which
	// carry their order permanently.
	FindBillBySellOrder(orderID uint) (*entity.Quotation, error)
}

type sellOrderRepository struct {
	db *gorm.DB
}

func NewSellOrderRepository(db *gorm.DB) SellOrderRepository {
	return &sellOrderRepository{db: db}
}

func (r *sellOrderRepository) Create(o *entity.SellOrder) error {
	return r.db.Create(o).Error
}

func (r *sellOrderRepository) scope(f SellOrderFilter) *gorm.DB {
	q := r.db.Model(&entity.SellOrder{})
	if f.UserID != nil {
		q = q.Where("sell_orders.user_id = ?", *f.UserID)
	}
	if f.Status != "" {
		q = q.Where("sell_orders.status = ?", f.Status)
	}
	if f.Search != "" {
		like := "%" + f.Search + "%"
		q = q.Joins("JOIN users ON users.id = sell_orders.user_id").
			Where("users.name ILIKE ? OR users.email ILIKE ?", like, like)
	}
	return q
}

func (r *sellOrderRepository) FindAll(f SellOrderFilter, page, limit int) ([]entity.SellOrder, int64, error) {
	var total int64
	if err := r.scope(f).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var orders []entity.SellOrder
	// Waiting orders first (that is what the customer acts on), then newest.
	err := r.scope(f).
		Preload("User").
		Preload("Bill").
		Order("CASE sell_orders.status WHEN 'active' THEN 0 WHEN 'filling' THEN 1 ELSE 2 END ASC").
		Order("sell_orders.created_at DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&orders).Error
	return orders, total, err
}

func (r *sellOrderRepository) FindByID(id uint) (*entity.SellOrder, error) {
	var o entity.SellOrder
	if err := r.db.Preload("User").Preload("Bill").First(&o, id).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *sellOrderRepository) ActiveTotalsByUser(userID uint) (ActiveTotals, error) {
	var t ActiveTotals
	err := r.db.Model(&entity.SellOrder{}).
		Select("COUNT(*) AS count, COALESCE(SUM(weight), 0) AS weight").
		Where("user_id = ? AND status IN ?", userID, []string{entity.SellOrderActive, entity.SellOrderFilling}).
		Scan(&t).Error
	return t, err
}

func (r *sellOrderRepository) Cancel(id uint, reason string) error {
	now := time.Now()
	return r.db.Model(&entity.SellOrder{}).
		// Only an order still waiting can be cancelled — a filling one is already
		// becoming a bill, and cancelling it would leave a bill with no order.
		Where("id = ? AND status = ?", id, entity.SellOrderActive).
		Updates(map[string]any{
			"status":        entity.SellOrderCancelled,
			"cancelled_at":  now,
			"cancel_reason": reason,
			"updated_at":    now,
		}).Error
}

func (r *sellOrderRepository) ClaimDue(price float64) ([]entity.SellOrder, error) {
	var claimed []entity.SellOrder
	// SKIP LOCKED so a row another transaction holds is left for the next tick
	// rather than blocking the whole sweep. Highest target first: if the caps are
	// ever hit mid-sweep, the orders closest to the market fill first.
	err := r.db.Raw(`
		UPDATE sell_orders SET status = ?, updated_at = NOW()
		WHERE id IN (
			SELECT id FROM sell_orders
			WHERE status = ? AND target_price <= ?
			ORDER BY target_price DESC, id ASC
			FOR UPDATE SKIP LOCKED
		)
		RETURNING *`,
		entity.SellOrderFilling, entity.SellOrderActive, price,
	).Scan(&claimed).Error
	return claimed, err
}

func (r *sellOrderRepository) MarkFilled(id uint, price float64, billID uint) error {
	now := time.Now()
	return r.db.Model(&entity.SellOrder{}).Where("id = ?", id).Updates(map[string]any{
		"status":       entity.SellOrderFilled,
		"filled_price": price,
		"filled_at":    now,
		"bill_id":      billID,
		"updated_at":   now,
	}).Error
}

func (r *sellOrderRepository) RecordFillPrice(id uint, price float64) error {
	return r.db.Model(&entity.SellOrder{}).
		Where("id = ? AND status = ?", id, entity.SellOrderFilling).
		Updates(map[string]any{"filled_price": price, "updated_at": time.Now()}).Error
}

func (r *sellOrderRepository) ReleaseClaim(id uint) error {
	return r.db.Model(&entity.SellOrder{}).
		Where("id = ? AND status = ?", id, entity.SellOrderFilling).
		// The price goes back with the claim: a released order never sold, and an
		// abandoned attempt's price left on the row would read as a fill on an order
		// that is still waiting.
		Updates(map[string]any{"status": entity.SellOrderActive, "filled_price": nil, "updated_at": time.Now()}).Error
}

func (r *sellOrderRepository) FindStuckFilling() ([]entity.SellOrder, error) {
	var orders []entity.SellOrder
	err := r.db.Where("status = ?", entity.SellOrderFilling).Find(&orders).Error
	return orders, err
}

func (r *sellOrderRepository) FindBillBySellOrder(orderID uint) (*entity.Quotation, error) {
	var bill entity.Quotation
	// Asked of the items, not of the bill: a fill joins the customer's open bill, so
	// several orders can share one, and the bill's own sell_order_id only remembers
	// whichever landed last. The item keeps its order for good.
	if err := r.db.
		Joins("JOIN quotation_items qi ON qi.quotation_id = quotations.id AND qi.deleted_at IS NULL").
		Where("qi.sell_order_id = ?", orderID).
		Order("quotations.id DESC").First(&bill).Error; err != nil {
		return nil, err
	}
	return &bill, nil
}
