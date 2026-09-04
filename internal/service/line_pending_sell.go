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
// It answers one question: does the SHOP have a full lot of metal sitting at
// รอออกบิล right now (65 บาท of gold, 1000 กรัม of silver — each traded as
// 1 กิโลกรัม)? Every bill still waiting to be issued counts towards the same
// pile, whoever sold it in: what the shop forwards to its counterparty is one
// kilo of metal, and it doesn't matter whether one customer or ten brought it
// in. Nothing carries over between days either — issuing the bills takes the
// weight out of the pile on its own.
//
// One rule per metal, for the same reason the backlog alert has one channel per
// metal: ทอง is weighed in บาท and เงิน in กรัม, and they are announced at
// different purities.
func keyPendingSellEnabled(metal string) string   { return "line_pending_sell_" + metal + "_enabled" }
func keyPendingSellThreshold(metal string) string { return "line_pending_sell_threshold_" + metal }
func keyPendingSellPurity(metal string) string    { return "line_pending_sell_purity_" + metal }

// KeyPendingSellName is who the announcement is signed by. The message is
// forwarded to a counterparty as-is, and the shop wants it to read as coming
// from one person — the owner — no matter which customer's metal made up the
// kilo. Editable on the settings page so a change of hands is a text box, not a
// deploy.
const KeyPendingSellName = "line_pending_sell_name"

// defaultPendingSellName is the owner as of the request that made this alert
// shop-wide; the config row seeded by migration 92 is what actually applies.
const defaultPendingSellName = "วีรชัย ชัยนุมาศ"

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

// SyncPendingSellAlert re-checks the shop's whole รอออกบิล pile for one metal
// and announces it when it reaches a whole lot that hasn't been announced yet.
//
// The latch is a lot COUNT, not a boolean: a pile that grows from 65 to 130 บาท
// gets a second message while ten sells that keep it between 65 and 70 get none.
// Every message reads "จำนวน 1 กิโลกรัม" — the count decides HOW MANY notices go
// out, never the number printed inside one. When the pile shrinks — staff
// removed a line, or the bills were issued — the count falls with it, which
// re-arms the alert for the next time it climbs.
//
// Safe to call in a goroutine; every failure path is silent by design (an alert
// must never break the request that caused it).
func SyncPendingSellAlert(db *gorm.DB, metal string) {
	metal = normalizeMetal(metal)
	if !IsLineMetal(metal) {
		return
	}
	threshold := pendingSellThreshold(db, metal)
	if threshold <= 0 {
		return
	}

	weight := PendingSellWeight(db, metal)
	lots := int(math.Floor((weight + pendingSellEpsilon) / threshold))

	alerted, err := swapPendingSellLots(db, metal, lots)
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
	// One notice per kilo that has just completed, never a running total: the
	// pile jumping straight from 0 to 2 kilos owes the counterparty two แจ้งขาย
	// of 1 กิโลกรัม, not a single line reading "2 กิโลกรัม".
	name := PendingSellName(db)
	purity := pendingSellPurity(db, metal)
	for i := alerted; i < lots; i++ {
		_ = linenotify.SendText(target, pendingSellMessage(name, bangkokNow(), metal, purity))
	}
}

// PendingSellWeight totals what the shop has taken in and not billed out: every
// bill still at รอออกบิล (10), for one metal, whoever it belongs to.
//
// Summed across every bill rather than per customer — the kilo the shop forwards
// is a kilo however many people brought it in, and one customer can anyway have
// an auto-sell fill open alongside the bill they have been adding to by hand.
// Legacy rows with no metal tag read as gold (see migration 85).
func PendingSellWeight(db *gorm.DB, metal string) float64 {
	bills := db.Model(&entity.Quotation{}).
		Where("is_bill = ? AND status = ?", true, billStatusPendingIssue).
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
// moment must not both read the old count and both decide to announce. That
// matters more now than it did per customer — every sell in the shop contends
// for the same row.
func swapPendingSellLots(db *gorm.DB, metal string, lots int) (previous int, err error) {
	err = db.Transaction(func(tx *gorm.DB) error {
		var row entity.LinePendingSellAlert
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&entity.LinePendingSellAlert{Metal: metal}).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("metal = ?", metal).First(&row).Error; err != nil {
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
//	วีรชัย ชัยนุมาศ วันที่ 31/07/2568 เวลา 14:35 น. แจ้งขายทองคำ 99.99% จำนวน 1 กิโลกรัม
//
// The quantity is always 1 กิโลกรัม and is not a parameter: the shop sells this
// on one kilo at a time, so each completed kilo gets its own notice rather than
// one notice carrying a running total. Two kilos completing together send two of
// these, which is what the counterparty is being told either way.
//
// The name is the shop's own (see KeyPendingSellName), not the customer's: the
// pile is everyone's metal together, and it is the shop that is selling it on.
func pendingSellMessage(sender string, at time.Time, metal, purity string) string {
	return fmt.Sprintf("%s วันที่ %02d/%02d/%d เวลา %s น. แจ้งขาย%s %s%% จำนวน 1 กิโลกรัม",
		sender, at.Day(), int(at.Month()), at.Year()+543,
		at.Format("15:04"), metalNoun(metal), purity)
}

// PendingSellName is who every pending-sell announcement is signed by.
func PendingSellName(db *gorm.DB) string {
	if name := strings.TrimSpace(configValue(db, KeyPendingSellName, "")); name != "" {
		return name
	}
	return defaultPendingSellName
}

// SetPendingSellName stores the signature. An empty string is ignored rather
// than saved: an unsigned announcement would go out reading " วันที่ ...".
func SetPendingSellName(db *gorm.DB, name string) {
	if name = strings.TrimSpace(name); name != "" {
		setConfigValue(db, KeyPendingSellName, name)
	}
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
// the live pile it is measured against — enough to tell at a glance whether the
// threshold is set sensibly and whether the next kilo is close.
type PendingSellStatus struct {
	Metal       string  `json:"metal"`
	Enabled     bool    `json:"enabled"`
	Threshold   float64 `json:"threshold"`
	Purity      string  `json:"purity"`
	Unit        string  `json:"unit"`         // บาท for gold, กรัม for silver
	Weight      float64 `json:"weight"`       // the whole shop's รอออกบิล pile
	BillCount   int64   `json:"bill_count"`   // how many bills make it up
	Lots        int     `json:"lots"`         // whole kilos that pile is worth
	AlertedLots int     `json:"alerted_lots"` // kilos already announced
}

// GetPendingSellStatus reports every metal's rule, in the same order as the
// backlog status so the settings page can lay the two sets of cards out alike.
func GetPendingSellStatus(db *gorm.DB) []PendingSellStatus {
	out := make([]PendingSellStatus, 0, len(LineMetals))
	for _, metal := range LineMetals {
		threshold := pendingSellThreshold(db, metal)
		weight := PendingSellWeight(db, metal)
		status := PendingSellStatus{
			Metal:     metal,
			Enabled:   configValue(db, keyPendingSellEnabled(metal), "false") == "true",
			Threshold: threshold,
			Purity:    pendingSellPurity(db, metal),
			Unit:      metalUnit(metal),
			Weight:    weight,
		}
		if threshold > 0 {
			status.Lots = int(math.Floor((weight + pendingSellEpsilon) / threshold))
		}

		db.Model(&entity.Quotation{}).
			Where("is_bill = ? AND status = ?", true, billStatusPendingIssue).
			Where("COALESCE(NULLIF(metal, ''), 'gold') = ?", metal).
			Count(&status.BillCount)

		var row entity.LinePendingSellAlert
		if err := db.Where("metal = ?", metal).First(&row).Error; err == nil {
			status.AlertedLots = row.AlertedLots
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
	msg := "[ทดสอบ] " + pendingSellMessage(PendingSellName(db), bangkokNow(), metal, pendingSellPurity(db, metal))
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
