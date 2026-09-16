package service

import (
	"errors"
	"testing"
	"time"
)

func storeLock(l *PriceLock) {
	lockMu.Lock()
	lockStore[l.ID] = l
	lockMu.Unlock()
}

func testLock(id string, userID uint, expiresIn time.Duration) *PriceLock {
	now := time.Now()
	return &PriceLock{
		ID: id, UserID: userID, CreatedAt: now, ExpiresAt: now.Add(expiresIn),
		TypeID: "15", TypeName: "ทองคำแท่ง 96.5%", Metal: "gold",
		Weight: 2, Price: 68278, PerGram: 4491.97, Total: 136556,
		PriceMode: PriceModeRealtime, RealtimeBuy: 68278, RealtimeSell: 68358,
	}
}

func TestPriceLockIsSingleUse(t *testing.T) {
	storeLock(testLock("lock-single", 5, 10*time.Second))

	got, err := ConsumePriceLock("lock-single", 5)
	if err != nil || got == nil || got.Price != 68278 {
		t.Fatalf("first confirm should hand back the locked price, got %+v err=%v", got, err)
	}
	// A replayed confirm must not sell the same locked price twice.
	if _, err := ConsumePriceLock("lock-single", 5); !errors.Is(err, ErrLockNotFound) {
		t.Errorf("second confirm err=%v, want ErrLockNotFound", err)
	}
}

func TestPriceLockBelongsToOneCustomer(t *testing.T) {
	storeLock(testLock("lock-owner", 5, 10*time.Second))

	if _, err := ConsumePriceLock("lock-owner", 23); !errors.Is(err, ErrLockNotFound) {
		t.Errorf("another customer's id err=%v, want ErrLockNotFound", err)
	}
	// ...and the attempt must not have burned the real customer's lock.
	if _, err := ConsumePriceLock("lock-owner", 5); err != nil {
		t.Errorf("owner confirm err=%v, want success", err)
	}
}

func TestPriceLockExpires(t *testing.T) {
	// Just past expiry but inside the round-trip grace: still honoured.
	storeLock(testLock("lock-grace", 5, -lockGrace/2))
	if _, err := ConsumePriceLock("lock-grace", 5); err != nil {
		t.Errorf("confirm within the grace window err=%v, want success", err)
	}

	storeLock(testLock("lock-late", 5, -(lockGrace + time.Second)))
	if _, err := ConsumePriceLock("lock-late", 5); !errors.Is(err, ErrLockExpired) {
		t.Errorf("late confirm err=%v, want ErrLockExpired", err)
	}

	if _, err := ConsumePriceLock("never-existed", 5); !errors.Is(err, ErrLockNotFound) {
		t.Errorf("unknown id err=%v, want ErrLockNotFound", err)
	}
}

func TestPriceLockItemCarriesTheLockedNumbers(t *testing.T) {
	lock := testLock("lock-item", 5, 10*time.Second)
	item := lock.Item()
	if item.Price != lock.Price || item.Total != lock.Total || item.PerGram != lock.PerGram ||
		item.Weight != lock.Weight || item.TypeID != lock.TypeID || item.Metal != lock.Metal {
		t.Errorf("bill line %+v does not match the lock %+v", item, lock)
	}
	if item.Plus != 0 {
		t.Errorf("locked line should carry no ราคาบวก, got %v", item.Plus)
	}
}

func TestExpiredLocksAreSweptAway(t *testing.T) {
	storeLock(testLock("lock-stale", 5, -2*time.Minute))
	lockMu.Lock()
	sweepExpiredLocked(time.Now())
	lockMu.Unlock()
	if _, ok := PeekPriceLock("lock-stale"); ok {
		t.Error("a lock nobody confirmed should not sit in memory for ever")
	}
}
