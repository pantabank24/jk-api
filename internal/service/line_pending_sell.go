package service

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"jk-api/internal/entity"
	"jk-api/pkg/linenotify"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Pending-sell alert — the second LINE alert, independent of the บิลค้างเคลียร์
// one in line_notify.go.
//
// It answers one question: does THIS customer have a full lot of metal sitting
// at รอออกบิล right now (65 บาท of gold, 1000 กรัม of silver — each traded as
// 1 กิโลกรัม)? Nothing accumulates across customers and nothing carries over
// between days: the pile is whatever they have sold in and not been billed for,
// and issuing the bill takes it out of the picture on its own.
//
// One rule per metal, for the same reason the backlog alert has one channel per
// metal: ทอง is weighed in บาท and เงิน in กรัม, and they are announced at
// different purities.
func keyPendingSellEnabled(metal string) string   { return "line_pending_sell_" + metal + "_enabled" }
func keyPendingSellThreshold(metal string) string { return "line_pending_sell_threshold_" + metal }
func keyPendingSellPurity(metal string) string    { return "line_pending_sell_purity_" + metal }

// pendingSellEpsilon absorbs the rounding of decimal(12,4) weights, so 64.9999999
// summed from stored items still counts as a full 65-บาท lot instead of sitting
// one ten-thousandth short forever.
const pendingSellEpsilon = 1e-6

// defaultPendingSellPurity is what the announcement says when nothing is
// configured: refined gold is 99.99%, refined silver 99.9%.
func defaultPendingSellPurity(metal string) string {
	if metal == "silver" {
		return "99.9"
	}
	return "99.99"
}

// metalNoun is how the announcement names the metal — "ทองคำ", not the "ทอง" the
// UI uses for column headers.
func metalNoun(metal string) string {
	if metal == "silver" {
		return "เงิน"
	}
	return "ทองคำ"
}

// SyncPendingSellAlert re-checks one customer's รอออกบิล pile for one metal and
// announces it when it reaches a whole lot that hasn't been announced yet.
//
// The latch is a lot COUNT, not a boolean: a customer who sells their way from
// 65 to 130 บาท gets a second message ("จำนวน 2 กิโลกรัม") while a customer
// clicking sell ten times between 65 and 70 gets none. When the pile shrinks —
// staff removed a line, or the bill was issued — the count falls with it, which
// re-arms the alert for the next time it climbs.
//
// Safe to call in a goroutine; every failure path is silent by design (an alert
// must never break the request that caused it).
func SyncPendingSellAlert(db *gorm.DB, userID uint, metal string) {
	metal = normalizeMetal(metal)
	if userID == 0 || !IsLineMetal(metal) {
		return
	}
	threshold := pendingSellThreshold(db, metal)
	if threshold <= 0 {
		return
	}

	weight := PendingSellWeight(db, userID, metal)
	lots := int(math.Floor((weight + pendingSellEpsilon) / threshold))

	alerted, err := swapPendingSellLots(db, userID, metal, lots)
	if err != nil || lots <= alerted {
		// Unchanged, or the pile shrank — the swap already re-armed the latch.
		return
	}

	// Announce only once the switches are on. The latch is written either way, so
	// turning the alert on doesn't immediately fire for piles that were already
	// over the line before anyone was listening.
	if configValue(db, KeyLineEnabled, "false") != "true" {
		return
	}
	if configValue(db, keyPendingSellEnabled(metal), "false") != "true" {
		return
	}
	target := configValue(db, KeyLineTargetID, "")
	if target == "" {
		return
	}
	_ = linenotify.SendText(target, pendingSellMessage(
		pendingSellCustomer(db, userID), bangkokNow(), metal, pendingSellPurity(db, metal), lots))
}

// PendingSellWeight totals what one customer has sold in and not been billed for:
// every bill of theirs still at รอออกบิล (10), for one metal.
//
// Summed across bills rather than read off a single one — a customer can have an
// auto-sell fill open alongside the bill they have been adding to by hand, and
// both are metal the shop is holding for them. Legacy rows with no metal tag read
// as gold (see migration 85).
func PendingSellWeight(db *gorm.DB, userID uint, metal string) float64 {
	bills := db.Model(&entity.Quotation{}).
		Where("is_bill = ? AND status = ? AND created_by = ?", true, billStatusPendingIssue, userID).
		Where("COALESCE(NULLIF(metal, ''), 'gold') = ?", metal)

	var agg struct{ Weight float64 }
	if err := db.Model(&entity.QuotationItem{}).
		Where("quotation_id IN (?)", bills.Session(&gorm.Session{}).Select("quotations.id")).
		Where("COALESCE(NULLIF(metal, ''), 'gold') = ?", metal).
		Select("COALESCE(SUM(weight), 0) AS weight").
		Scan(&agg).Error; err != nil {
		return 0
	}
	return agg.Weight
}

// swapPendingSellLots stores the new lot count and returns what it was before.
// The read-modify-write runs under a row lock: two sells landing at the same
// moment must not both read the old count and both decide to announce.
func swapPendingSellLots(db *gorm.DB, userID uint, metal string, lots int) (previous int, err error) {
	err = db.Transaction(func(tx *gorm.DB) error {
		var row entity.LinePendingSellAlert
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&entity.LinePendingSellAlert{UserID: userID, Metal: metal}).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND metal = ?", userID, metal).First(&row).Error; err != nil {
			return err
		}
		previous = row.AlertedLots
		if previous == lots {
			return nil
		}
		return tx.Model(&entity.LinePendingSellAlert{}).
			Where("id = ?", row.ID).Update("alerted_lots", lots).Error
	})
	if err != nil {
		return 0, err
	}
	return previous, nil
}

// pendingSellMessage renders the announcement exactly as the shop dictates it —
// plain text, no Flex card, because it is forwarded to a counterparty as-is:
//
//	วีระชัย ชัยนุมาศ วันที่ 31/07/2568 เวลา 14:35 น. แจ้งขายทองคำ 99.99% จำนวน 1 กิโลกรัม
func pendingSellMessage(customer string, at time.Time, metal, purity string, lots int) string {
	return fmt.Sprintf("%s วันที่ %02d/%02d/%d เวลา %s น. แจ้งขาย%s %s%% จำนวน %s กิโลกรัม",
		customer, at.Day(), int(at.Month()), at.Year()+543,
		at.Format("15:04"), metalNoun(metal), purity, formatInt(int64(lots)))
}

// pendingSellCustomer names the customer the pile belongs to. Bills are created
// under the customer's own account even when staff sell on their behalf, so the
// user row is the right name to announce.
func pendingSellCustomer(db *gorm.DB, userID uint) string {
	var user entity.User
	if err := db.Select("name").First(&user, userID).Error; err == nil {
		if name := strings.TrimSpace(user.Name); name != "" {
			return name
		}
	}
	var member entity.Member
	if err := db.Select("fname", "lname").Where("user_id = ?", userID).First(&member).Error; err == nil {
		if name := strings.TrimSpace(member.Fname + " " + member.Lname); name != "" {
			return name
		}
	}
	return "ไม่ระบุลูกค้า"
}

func pendingSellThreshold(db *gorm.DB, metal string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(configValue(db, keyPendingSellThreshold(metal), "0")), 64)
	return v
}

// pendingSellPurity is free text on purpose: the shop types what the counterparty
// expects to read ("99.99", "96.5"), and it goes into the message untouched.
func pendingSellPurity(db *gorm.DB, metal string) string {
	p := strings.TrimSpace(configValue(db, keyPendingSellPurity(metal), ""))
	p = strings.TrimSpace(strings.TrimSuffix(p, "%"))
	if p == "" {
		return defaultPendingSellPurity(metal)
	}
	return p
}

// PendingSellStatus is what the settings page shows for one metal: the rule, plus
// how many customers are over the line right now and the biggest pile among them
// — enough to tell at a glance whether the threshold is set sensibly.
type PendingSellStatus struct {
	Metal     string  `json:"metal"`
	Enabled   bool    `json:"enabled"`
	Threshold float64 `json:"threshold"`
	Purity    string  `json:"purity"`
	Unit      string  `json:"unit"`       // บาท for gold, กรัม for silver
	OverCount int64   `json:"over_count"` // customers at or above the threshold
	TopWeight float64 `json:"top_weight"` // largest single customer's pending pile
	TopName   string  `json:"top_name"`   // and who it belongs to
}

// GetPendingSellStatus reports every metal's rule, in the same order as the
// backlog status so the settings page can lay the two sets of cards out alike.
func GetPendingSellStatus(db *gorm.DB) []PendingSellStatus {
	out := make([]PendingSellStatus, 0, len(LineMetals))
	for _, metal := range LineMetals {
		threshold := pendingSellThreshold(db, metal)
		status := PendingSellStatus{
			Metal:     metal,
			Enabled:   configValue(db, keyPendingSellEnabled(metal), "false") == "true",
			Threshold: threshold,
			Purity:    pendingSellPurity(db, metal),
			Unit:      metalUnit(metal),
		}

		// One row per customer with a รอออกบิล pile of this metal, heaviest first.
		var piles []struct {
			Name   string
			Weight float64
		}
		db.Model(&entity.QuotationItem{}).
			Joins("JOIN quotations ON quotations.id = quotation_items.quotation_id AND quotations.deleted_at IS NULL").
			Joins("LEFT JOIN users ON users.id = quotations.created_by").
			Where("quotations.is_bill = ? AND quotations.status = ?", true, billStatusPendingIssue).
			Where("COALESCE(NULLIF(quotations.metal, ''), 'gold') = ?", metal).
			Where("COALESCE(NULLIF(quotation_items.metal, ''), 'gold') = ?", metal).
			Where("quotation_items.deleted_at IS NULL").
			Group("quotations.created_by, users.name").
			Select("users.name AS name, COALESCE(SUM(quotation_items.weight), 0) AS weight").
			Order("weight DESC").
			Scan(&piles)

		if len(piles) > 0 {
			status.TopWeight = piles[0].Weight
			status.TopName = strings.TrimSpace(piles[0].Name)
		}
		if threshold > 0 {
			for _, p := range piles {
				if p.Weight+pendingSellEpsilon >= threshold {
					status.OverCount++
				}
			}
		}
		out = append(out, status)
	}
	return out
}

// SetPendingSellConfig writes one metal's rule. Purity is stored as typed (minus a
// trailing %), because it is copy shown to a counterparty, not a number the
// system computes with.
func SetPendingSellConfig(db *gorm.DB, metal string, enabled *bool, threshold *float64, purity *string) {
	if !IsLineMetal(metal) {
		return
	}
	if enabled != nil {
		setConfigValue(db, keyPendingSellEnabled(metal), boolConfigValue(*enabled))
	}
	if threshold != nil {
		setConfigValue(db, keyPendingSellThreshold(metal), strconv.FormatFloat(*threshold, 'f', -1, 64))
	}
	if purity != nil {
		setConfigValue(db, keyPendingSellPurity(metal), strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(*purity), "%")))
	}
}

// SendPendingSellTestAlert pushes the announcement the shop would receive for one
// lot, so the wiring can be verified without waiting for a customer to reach the
// threshold. It does not touch any latch.
func SendPendingSellTestAlert(db *gorm.DB, metal string) error {
	if !IsLineMetal(metal) {
		return fmt.Errorf("metal ไม่ถูกต้อง")
	}
	target := configValue(db, KeyLineTargetID, "")
	if target == "" {
		return fmt.Errorf("ยังไม่ได้เชื่อมต่อ LINE")
	}
	msg := "[ทดสอบ] " + pendingSellMessage("ชื่อลูกค้า", bangkokNow(), metal, pendingSellPurity(db, metal), 1)
	return linenotify.SendText(target, msg)
}

// normalizeMetal mirrors the COALESCE/NULLIF fallback used across the bill
// queries: an untagged legacy row is gold.
func normalizeMetal(metal string) string {
	if strings.TrimSpace(metal) == "" {
		return "gold"
	}
	return metal
}

// IsLineMetal reports whether a metal has alert channels at all. Platinum and
// palladium are sellable but have no LINE rules, so they simply never alert —
// and a settings save can't invent a rule for them either.
func IsLineMetal(metal string) bool {
	for _, m := range LineMetals {
		if m == metal {
			return true
		}
	}
	return false
}

func boolConfigValue(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
