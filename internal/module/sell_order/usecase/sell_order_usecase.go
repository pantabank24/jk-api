package usecase

import (
	"errors"
	"fmt"
	"math"

	"jk-api/internal/entity"
	"jk-api/internal/module/sell_order/repository"
	"jk-api/internal/service"

	"gorm.io/gorm"
)

type SellOrderUsecase interface {
	Create(req *CreateSellOrderRequest) (*entity.SellOrder, error)
	List(f repository.SellOrderFilter, page, limit int) ([]entity.SellOrder, int64, error)
	GetByID(id uint) (*entity.SellOrder, error)
	// Cancel withdraws a waiting order. actorID is recorded in the reason when a
	// staff member cancels on the customer's behalf.
	Cancel(id uint, byStaff bool, reason string) (*entity.SellOrder, error)
}

type CreateSellOrderRequest struct {
	// UserID/CreatedBy/Store/Branch come from the JWT context (or the chosen
	// customer for a staff-placed order) — never from the payload.
	UserID    uint  `json:"-"`
	CreatedBy uint  `json:"-"`
	StoreID   *uint `json:"-"`
	BranchID  *uint `json:"-"`

	Weight      float64 `json:"weight"`
	TargetPrice float64 `json:"target_price"`
	Note        string  `json:"note"`
}

type sellOrderUsecase struct {
	repo repository.SellOrderRepository
	db   *gorm.DB
}

func NewSellOrderUsecase(repo repository.SellOrderRepository, db *gorm.DB) SellOrderUsecase {
	return &sellOrderUsecase{repo: repo, db: db}
}

// resolveGoldBarType finds the gold-bar 96.5% type used to price a sale, matching
// it by name rather than a hardcoded id (ids differ across environments and
// reseeds) — the same rule the sell screen uses (isGoldBar in billCalculate.tsx).
func (u *sellOrderUsecase) resolveGoldBarType() (*entity.GoldType, error) {
	var gt entity.GoldType
	err := u.db.Where("metal = ? AND is_active = ? AND name LIKE ? AND name LIKE ?",
		"gold", true, "%แท่ง%", "%96.5%").
		Order("sort_order ASC").First(&gt).Error
	if err != nil {
		return nil, errors.New("ไม่พบประเภททองคำแท่ง 96.5% ในระบบ")
	}
	return &gt, nil
}

func (u *sellOrderUsecase) Create(req *CreateSellOrderRequest) (*entity.SellOrder, error) {
	cfg := service.GetAutoSellConfig(u.db)
	if !cfg.Enabled {
		return nil, errors.New("ระบบขายอัตโนมัติปิดอยู่ ไม่สามารถตั้งคำสั่งขายได้")
	}

	status := service.GetAutoSellStatus(u.db)
	if err := validateWeight(req.Weight, status); err != nil {
		return nil, err
	}
	if req.TargetPrice <= 0 {
		return nil, errors.New("กรุณาระบุราคาเป้าหมาย")
	}

	pricing := service.GetRealtimePricing(u.db)

	// A target at or below the market would fire on the next tick, which is not
	// what "ตั้งราคาขาย" means — the customer almost certainly wants the normal
	// sell screen instead, so say so rather than silently selling for them.
	var priceNow float64
	if status.Price != nil {
		priceNow = *status.Price
		if req.TargetPrice <= priceNow {
			return nil, fmt.Errorf("ราคาเป้าหมายต้องสูงกว่าราคารับซื้อปัจจุบัน (%s บาท) — ถ้าต้องการขายที่ราคานี้เลย ให้ใช้หน้าขายปกติ",
				formatPrice(priceNow))
		}
	}

	// Caps: nothing reserves the customer's gold, so the only protection against a
	// price spike filling a stack of orders they cannot deliver on is a limit on
	// how much can be waiting at once.
	totals, err := u.repo.ActiveTotalsByUser(req.UserID)
	if err != nil {
		return nil, err
	}
	if int(totals.Count) >= cfg.MaxActiveOrders {
		return nil, fmt.Errorf("ตั้งคำสั่งขายค้างไว้ได้ไม่เกิน %d รายการ (ขณะนี้มี %d รายการ) กรุณายกเลิกรายการเดิมก่อน",
			cfg.MaxActiveOrders, totals.Count)
	}
	if totals.Weight+req.Weight > cfg.MaxActiveWeight {
		return nil, fmt.Errorf("น้ำหนักรวมของคำสั่งขายที่รออยู่ต้องไม่เกิน %s บาท (ขณะนี้ %s บาท)",
			formatWeight(cfg.MaxActiveWeight), formatWeight(totals.Weight))
	}

	goldBar, err := u.resolveGoldBarType()
	if err != nil {
		return nil, err
	}

	createdBy := req.CreatedBy
	order := &entity.SellOrder{
		UserID:      req.UserID,
		CreatedBy:   &createdBy,
		StoreID:     req.StoreID,
		BranchID:    req.BranchID,
		Metal:       "gold",
		GoldTypeID:  &goldBar.ID,
		TypeName:    goldBar.Name,
		Weight:      req.Weight,
		TargetPrice: req.TargetPrice,
		Status:      entity.SellOrderActive,
		// Audit trail for the spread question: if the shop retunes premium/spread
		// later, this is the only record of what the customer was quoted when they
		// picked this target.
		PremiumAtCreate: pricing.Premium,
		SpreadAtCreate:  pricing.Spread,
		PriceAtCreate:   priceNow,
		Note:            req.Note,
	}
	if err := u.repo.Create(order); err != nil {
		return nil, err
	}
	return u.repo.FindByID(order.ID)
}

// validateWeight applies the same bounds the sell screen's stepper enforces: a
// whole number of baht-weight, and a multiple of 5 unless the custom-weight
// schedule currently lets customers type any weight.
func validateWeight(w float64, status service.AutoSellStatus) error {
	if w <= 0 {
		return errors.New("กรุณาระบุน้ำหนัก")
	}
	if w != math.Trunc(w) {
		return errors.New("น้ำหนักต้องเป็นจำนวนเต็ม (บาท)")
	}
	if w < status.WeightMin || w > status.WeightMax {
		return fmt.Errorf("น้ำหนักต้องอยู่ระหว่าง %s ถึง %s บาท",
			formatWeight(status.WeightMin), formatWeight(status.WeightMax))
	}
	if status.WeightStep > 0 && math.Mod(w, status.WeightStep) != 0 {
		return fmt.Errorf("น้ำหนักต้องเป็นจำนวนเท่าของ %s บาท", formatWeight(status.WeightStep))
	}
	return nil
}

func (u *sellOrderUsecase) List(f repository.SellOrderFilter, page, limit int) ([]entity.SellOrder, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return u.repo.FindAll(f, page, limit)
}

func (u *sellOrderUsecase) GetByID(id uint) (*entity.SellOrder, error) {
	return u.repo.FindByID(id)
}

func (u *sellOrderUsecase) Cancel(id uint, byStaff bool, reason string) (*entity.SellOrder, error) {
	order, err := u.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("ไม่พบคำสั่งขายนี้")
	}
	switch order.Status {
	case entity.SellOrderFilled:
		return nil, errors.New("คำสั่งขายนี้ขายไปแล้ว ยกเลิกไม่ได้")
	case entity.SellOrderCancelled:
		return nil, errors.New("คำสั่งขายนี้ถูกยกเลิกไปแล้ว")
	case entity.SellOrderFilling:
		return nil, errors.New("คำสั่งขายนี้กำลังทำรายการขาย ยกเลิกไม่ได้")
	}
	if reason == "" {
		if byStaff {
			reason = "ยกเลิกโดยเจ้าหน้าที่"
		} else {
			reason = "ยกเลิกโดยลูกค้า"
		}
	}
	if err := u.repo.Cancel(id, reason); err != nil {
		return nil, err
	}
	return u.repo.FindByID(id)
}

func formatPrice(v float64) string  { return fmt.Sprintf("%.2f", v) }
func formatWeight(v float64) string { return fmt.Sprintf("%g", v) }
