package service

import (
	"errors"
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

// Sell-in accumulation alert — the second LINE alert, independent of the
// บิลค้างเคลียร์ one in line_notify.go.
//
// The shop buys metal from many customers in small lots. Once those add up to a
// full lot (65 บาท of gold, 1000 กรัม of silver — each traded as 1 กิโลกรัม) it
// announces the sale onward, so the meter runs across ALL customers of the shop,
// not per customer and not per bill.
//
// One meter per metal, for the same reason the backlog alert has one channel per
// metal: ทอง is weighed in บาท and เงิน in กรัม, and they are announced at
// different purities, so a shared meter could not describe either. Bills are
// single-metal, so every bill lands on exactly one meter.
//
// Metal enters at ออกบิล (สถานะ 11 รอตรวจบิล) — the point the weight is fixed and
// handed to the customer.
func keySellAccumEnabled(metal string) string   { return "line_sell_accum_" + metal + "_enabled" }
func keySellAccumThreshold(metal string) string { return "line_sell_accum_threshold_" + metal }
func keySellAccumPurity(metal string) string    { return "line_sell_accum_purity_" + metal }
func keySellAccumWeight(metal string) string    { return "line_sell_accum_weight_" + metal }

// sellAccumEpsilon absorbs the rounding of decimal(12,4) weights, so 64.9999999
// summed from stored items still counts as a full 65-บาท lot instead of parking
// a lot forever one ten-thousandth short.
const sellAccumEpsilon = 1e-6

// defaultSellAccumPurity is what the announcement says when nothing is
// configured: refined gold is 99.99%, refined silver 99.9%.
func defaultSellAccumPurity(metal string) string {
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

// AccumulateSellIn adds one just-issued bill's weight to its metal's meter and
// announces every full lot it completes. Announcing is one message even when a
// single bill completes several lots ("จำนวน 3 กิโลกรัม") — the shop reads one
// line, and the monthly push quota is not spent three times over.
//
// Safe to call in a goroutine; every failure path is silent by design (an alert
// must never break the request that caused it).
func AccumulateSellIn(db *gorm.DB, bill *entity.Quotation) {
	if bill == nil {
		return
	}
	metal := normalizeMetal(bill.Metal)
	if !IsLineMetal(metal) {
		return
	}
	threshold := sellAccumThreshold(db, metal)
	if threshold <= 0 {
		return
	}
	// Off means off: the meter does not run either, so switching the alert on
	// later starts from today rather than firing a burst for metal bought while
	// nobody was listening.
	if configValue(db, KeyLineEnabled, "false") != "true" {
		return
	}
	if configValue(db, keySellAccumEnabled(metal), "false") != "true" {
		return
	}

	weight := billMetalWeight(db, bill.ID, metal)
	if weight <= 0 {
		return
	}

	lots, _, err := consumeSellAccum(db, metal, weight, threshold)
	if err != nil || lots < 1 {
		return
	}

	target := configValue(db, KeyLineTargetID, "")
	if target == "" {
		return
	}
	_ = linenotify.SendText(target, sellAccumMessage(
		sellAccumCustomer(db, bill), bangkokNow(), metal, sellAccumPurity(db, metal), lots))
}

// ReleaseSellIn takes a bill's weight back off its metal's meter when an issued
// bill is pulled back to รอออกบิล or cancelled. Without it a re-issued bill would
// be counted twice — the same reason RevertIssuance undoes the balance ledger.
// An announcement already sent stands; only the meter is corrected.
func ReleaseSellIn(db *gorm.DB, bill *entity.Quotation) {
	if bill == nil {
		return
	}
	metal := normalizeMetal(bill.Metal)
	if !IsLineMetal(metal) {
		return
	}
	weight := billMetalWeight(db, bill.ID, metal)
	if weight <= 0 {
		return
	}
	// threshold 0 → subtract only, never announce on the way down.
	_, _, _ = consumeSellAccum(db, metal, -weight, 0)
}

// billMetalWeight sums one bill's lines for the given metal, read back from the
// items table rather than from a preloaded association so a caller that passed a
// partially loaded bill still gets the real weight. Legacy rows with no metal tag
// read as gold (see migration 85).
func billMetalWeight(db *gorm.DB, billID uint, metal string) float64 {
	var agg struct{ Weight float64 }
	if err := db.Model(&entity.QuotationItem{}).
		Where("quotation_id = ?", billID).
		Where("COALESCE(NULLIF(metal, ''), 'gold') = ?", metal).
		Select("COALESCE(SUM(weight), 0) AS weight").
		Scan(&agg).Error; err != nil {
		return 0
	}
	return agg.Weight
}

// consumeSellAccum applies delta to one metal's meter and takes out as many whole
// lots as fit, returning the lot count and what is left over. The
// read-modify-write runs under a row lock: two masters issuing bills at the same
// moment must not each read the old total and write their own sum over the
// other's.
//
// threshold <= 0 only accumulates (no lots come out), which is how ReleaseSellIn
// subtracts without announcing.
func consumeSellAccum(db *gorm.DB, metal string, delta, threshold float64) (lots int, remainder float64, err error) {
	key := keySellAccumWeight(metal)
	err = db.Transaction(func(tx *gorm.DB) error {
		current, lockErr := lockSellAccum(tx, key)
		if lockErr != nil {
			return lockErr
		}
		current += delta
		if current < 0 {
			current = 0
		}
		if threshold > 0 {
			lots = int(math.Floor((current + sellAccumEpsilon) / threshold))
			if lots > 0 {
				current -= float64(lots) * threshold
			}
			if current < 0 {
				current = 0
			}
		}
		remainder = current
		return tx.Model(&entity.SystemConfig{}).
			Where("key = ?", key).
			Update("value", strconv.FormatFloat(current, 'f', 4, 64)).Error
	})
	if err != nil {
		return 0, 0, err
	}
	return lots, remainder, nil
}

// lockSellAccum reads a meter FOR UPDATE, seeding the row when the migration that
// creates it hasn't run yet. The seed is written with ON CONFLICT DO NOTHING so
// two concurrent first-ever bills don't collide on the unique key.
func lockSellAccum(tx *gorm.DB, key string) (float64, error) {
	read := func() (entity.SystemConfig, error) {
		var cfg entity.SystemConfig
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("key = ?", key).First(&cfg).Error
		return cfg, err
	}

	cfg, err := read()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&entity.SystemConfig{
				Key:         key,
				Value:       "0",
				Description: "LINE: ภายใน — น้ำหนักสะสมที่ยังไม่ครบเกณฑ์",
			}).Error; err != nil {
			return 0, err
		}
		cfg, err = read()
	}
	if err != nil {
		return 0, err
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(cfg.Value), 64)
	return v, nil
}

// sellAccumMessage renders the announcement exactly as the shop dictates it —
// plain text, no Flex card, because it is forwarded to a counterparty as-is:
//
//	วีระชัย ชัยนุมาศ วันที่ 31/07/2568 เวลา 14:35 น. แจ้งขายทองคำ 99.99% จำนวน 1 กิโลกรัม
func sellAccumMessage(customer string, at time.Time, metal, purity string, lots int) string {
	return fmt.Sprintf("%s วันที่ %02d/%02d/%d เวลา %s น. แจ้งขาย%s %s%% จำนวน %s กิโลกรัม",
		customer, at.Day(), int(at.Month()), at.Year()+543,
		at.Format("15:04"), metalNoun(metal), purity, formatInt(int64(lots)))
}

// sellAccumCustomer names the customer whose bill completed the lot. The bill's
// creator IS the customer here (they raise their own sell), with the member
// profile as the fallback for walk-ins booked by staff.
func sellAccumCustomer(db *gorm.DB, bill *entity.Quotation) string {
	if bill.Creator != nil && strings.TrimSpace(bill.Creator.Name) != "" {
		return strings.TrimSpace(bill.Creator.Name)
	}
	if bill.Member != nil {
		if name := strings.TrimSpace(bill.Member.Fname + " " + bill.Member.Lname); name != "" {
			return name
		}
	}
	// Relations weren't preloaded — go and get the name rather than announce an
	// anonymous sale.
	if bill.CreatedBy != nil {
		var user entity.User
		if err := db.Select("name").First(&user, *bill.CreatedBy).Error; err == nil {
			if name := strings.TrimSpace(user.Name); name != "" {
				return name
			}
		}
	}
	if bill.MemberID != nil {
		var member entity.Member
		if err := db.Select("fname", "lname").First(&member, *bill.MemberID).Error; err == nil {
			if name := strings.TrimSpace(member.Fname + " " + member.Lname); name != "" {
				return name
			}
		}
	}
	return "ไม่ระบุลูกค้า"
}

func sellAccumThreshold(db *gorm.DB, metal string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(configValue(db, keySellAccumThreshold(metal), "0")), 64)
	return v
}

// sellAccumPurity is free text on purpose: the shop types what the counterparty
// expects to read ("99.99", "96.5"), and it goes into the message untouched.
func sellAccumPurity(db *gorm.DB, metal string) string {
	p := strings.TrimSpace(configValue(db, keySellAccumPurity(metal), ""))
	p = strings.TrimSpace(strings.TrimSuffix(p, "%"))
	if p == "" {
		return defaultSellAccumPurity(metal)
	}
	return p
}

// SellAccumStatus is what the settings page shows for one metal: the configured
// rule next to the live meter, so the shop can see how far the next announcement
// is.
type SellAccumStatus struct {
	Metal       string  `json:"metal"`
	Enabled     bool    `json:"enabled"`
	Threshold   float64 `json:"threshold"`
	Purity      string  `json:"purity"`
	Unit        string  `json:"unit"`        // บาท for gold, กรัม for silver
	Accumulated float64 `json:"accumulated"` // on the meter right now
	Remaining   float64 `json:"remaining"`   // still needed for the next lot
}

// GetSellAccumStatus reports every metal's meter, in the same order as the
// backlog status so the settings page can lay the two sets of cards out alike.
func GetSellAccumStatus(db *gorm.DB) []SellAccumStatus {
	out := make([]SellAccumStatus, 0, len(LineMetals))
	for _, metal := range LineMetals {
		threshold := sellAccumThreshold(db, metal)
		accumulated, _ := strconv.ParseFloat(strings.TrimSpace(configValue(db, keySellAccumWeight(metal), "0")), 64)
		remaining := 0.0
		if threshold > 0 {
			remaining = threshold - accumulated
			if remaining < 0 {
				remaining = 0
			}
		}
		out = append(out, SellAccumStatus{
			Metal:       metal,
			Enabled:     configValue(db, keySellAccumEnabled(metal), "false") == "true",
			Threshold:   threshold,
			Purity:      sellAccumPurity(db, metal),
			Unit:        metalUnit(metal),
			Accumulated: accumulated,
			Remaining:   remaining,
		})
	}
	return out
}

// SetSellAccumConfig writes one metal's rule. Purity is stored as typed (minus a
// trailing %), because it is copy shown to a counterparty, not a number the
// system computes with.
func SetSellAccumConfig(db *gorm.DB, metal string, enabled *bool, threshold *float64, purity *string) {
	if !IsLineMetal(metal) {
		return
	}
	if enabled != nil {
		setConfigValue(db, keySellAccumEnabled(metal), boolConfigValue(*enabled))
	}
	if threshold != nil {
		setConfigValue(db, keySellAccumThreshold(metal), strconv.FormatFloat(*threshold, 'f', -1, 64))
	}
	if purity != nil {
		setConfigValue(db, keySellAccumPurity(metal), strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(*purity), "%")))
	}
}

// ResetSellAccum zeroes one metal's meter — the manual correction for when the
// shop has settled a lot outside the system and the carried remainder no longer
// matches what is on the bench.
func ResetSellAccum(db *gorm.DB, metal string) {
	if !IsLineMetal(metal) {
		return
	}
	setConfigValue(db, keySellAccumWeight(metal), "0")
}

// SendSellAccumTestAlert pushes the announcement the shop would receive for one
// lot of the given metal, so the wiring can be verified without waiting for a
// full lot to come in. It does not touch the meter.
func SendSellAccumTestAlert(db *gorm.DB, metal string) error {
	if !IsLineMetal(metal) {
		return fmt.Errorf("metal ไม่ถูกต้อง")
	}
	target := configValue(db, KeyLineTargetID, "")
	if target == "" {
		return fmt.Errorf("ยังไม่ได้เชื่อมต่อ LINE")
	}
	msg := "[ทดสอบ] " + sellAccumMessage("ชื่อลูกค้า", bangkokNow(), metal, sellAccumPurity(db, metal), 1)
	return linenotify.SendText(target, msg)
}

// normalizeMetal mirrors the SQL COALESCE(NULLIF(metal,''),'gold') used across
// the bill queries: an untagged legacy row is gold.
func normalizeMetal(metal string) string {
	if strings.TrimSpace(metal) == "" {
		return "gold"
	}
	return metal
}

// IsLineMetal reports whether a metal has alert channels at all. Platinum and
// palladium are sellable but have no LINE rules, so they simply never accumulate
// — and a settings save can't invent a meter for them either.
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
