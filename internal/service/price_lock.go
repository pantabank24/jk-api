package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	billUC "jk-api/internal/module/bill/usecase"

	"gorm.io/gorm"
)

// KeySellPriceLockSeconds is how long a locked price stays good. It is also the
// countdown the customer sees on the confirm dialog, so changing it changes both.
const KeySellPriceLockSeconds = "sell_price_lock_seconds"

// Bounds for that setting. Long locks hand the customer a free option on a
// moving market; very short ones are unfair to anyone reading the dialog.
const (
	SellPriceLockSecondsMin     = 3.0
	SellPriceLockSecondsMax     = 60.0
	defaultSellPriceLockSeconds = 10.0
)

// lockGrace absorbs the round trip between the customer's press and the request
// landing, so a lock never expires in flight for someone who answered in time.
const lockGrace = 2 * time.Second

// PriceLock is a price the shop has committed to for one sale: the server
// derived it from its own feed when the customer pressed ส่งขาย, showed it to
// them, and will honour exactly that figure if they confirm in time. The
// browser never sends a price at all, so there is nothing to tamper with.
type PriceLock struct {
	ID        string
	UserID    uint
	CreatedAt time.Time
	ExpiresAt time.Time

	GoldTypeID uint
	TypeID     string
	TypeName   string
	Metal      string
	Weight     float64
	Percent    float64
	Price      float64
	PerGram    float64
	Total      float64
	PriceMode  string
	// RealtimeBuy/RealtimeSell are the reading the price came from, kept so the
	// bill's price round records the very quote that was honoured.
	RealtimeBuy  float64
	RealtimeSell float64
}

// Item turns the lock into the bill line it stands for.
func (l *PriceLock) Item() billUC.CreateBillItemRequest {
	return billUC.CreateBillItemRequest{
		TypeID:   l.TypeID,
		TypeName: l.TypeName,
		Metal:    l.Metal,
		Plus:     0,
		Price:    l.Price,
		Percent:  l.Percent,
		Weight:   l.Weight,
		PerGram:  l.PerGram,
		Total:    l.Total,
	}
}

// SecondsLeft is what the dialog counts down.
func (l *PriceLock) SecondsLeft() int {
	s := int(time.Until(l.ExpiresAt).Round(time.Second) / time.Second)
	if s < 0 {
		return 0
	}
	return s
}

var (
	lockMu    sync.Mutex
	lockStore = map[string]*PriceLock{}
)

// Errors a caller turns into a message for the customer.
var (
	ErrLockNotFound = errors.New("ไม่พบราคาที่ล็อกไว้ กรุณากดส่งขายใหม่")
	ErrLockExpired  = errors.New("หมดเวลาราคาที่ล็อกไว้ กรุณากดส่งขายใหม่")
)

// SellPriceLockSeconds reads the configured lock window, clamped on read like
// the other pricing settings.
func SellPriceLockSeconds(db *gorm.DB) float64 {
	return clamp(configFloat(db, KeySellPriceLockSeconds, defaultSellPriceLockSeconds),
		SellPriceLockSecondsMin, SellPriceLockSecondsMax)
}

// NewPriceLock prices a sale from the server's own feed and holds that figure
// for the configured window. Returns the rejection as-is when the sale itself
// is not allowed (closed, wrong type, bad weight), so the customer is told why.
func NewPriceLock(db *gorm.DB, userID uint, req SellQuoteRequest) (*PriceLock, error) {
	quote, err := QuoteSell(db, req)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	lock := &PriceLock{
		ID:           newLockID(),
		UserID:       userID,
		CreatedAt:    now,
		ExpiresAt:    now.Add(time.Duration(SellPriceLockSeconds(db)) * time.Second),
		GoldTypeID:   quote.GoldTypeID,
		TypeID:       quote.TypeID,
		TypeName:     quote.TypeName,
		Metal:        quote.Metal,
		Weight:       quote.Weight,
		Percent:      quote.Percent,
		Price:        quote.Price,
		PerGram:      quote.PerGram,
		Total:        quote.Total,
		PriceMode:    quote.PriceMode,
		RealtimeBuy:  quote.RealtimeBuy,
		RealtimeSell: quote.RealtimeSell,
	}

	lockMu.Lock()
	sweepExpiredLocked(now)
	lockStore[lock.ID] = lock
	lockMu.Unlock()
	return lock, nil
}

// ConsumePriceLock hands back the lock and retires it in the same step, so a
// replayed confirm cannot sell the same locked price twice.
func ConsumePriceLock(id string, userID uint) (*PriceLock, error) {
	lockMu.Lock()
	defer lockMu.Unlock()

	lock, ok := lockStore[id]
	// A lock belongs to the customer it was quoted for; anyone else's id reads as
	// "not found" rather than telling them it exists.
	if !ok || lock.UserID != userID {
		return nil, ErrLockNotFound
	}
	delete(lockStore, id)
	if time.Now().After(lock.ExpiresAt.Add(lockGrace)) {
		return nil, ErrLockExpired
	}
	return lock, nil
}

// PeekPriceLock reports a lock without consuming it (tests and diagnostics).
func PeekPriceLock(id string) (*PriceLock, bool) {
	lockMu.Lock()
	defer lockMu.Unlock()
	lock, ok := lockStore[id]
	return lock, ok
}

// sweepExpiredLocked drops locks nobody confirmed. Called while holding the
// mutex, on every new lock — the store only grows one entry per ส่งขาย press.
func sweepExpiredLocked(now time.Time) {
	for id, lock := range lockStore {
		if now.After(lock.ExpiresAt.Add(time.Minute)) {
			delete(lockStore, id)
		}
	}
}

func newLockID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing is not recoverable here; a time-based id would be
		// guessable, and a guessable id is somebody else's price.
		panic("price lock: " + err.Error())
	}
	return hex.EncodeToString(buf)
}
