package documentcode

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// NextBill returns the next shared BILL number. Bills and non-master quotations
// use one namespace because quotations.code is unique across both document kinds.
func NextBill(db *gorm.DB) (string, error) {
	return nextNumeric(db, "BILL", 4)
}

// NextAdmin returns the next monthly P number used only for documents whose
// originating creator is the master/admin.
func NextAdmin(db *gorm.DB) (string, error) {
	now := time.Now()
	buddhistYear := now.Year() + 543
	prefix := fmt.Sprintf("P%02d%02d", buddhistYear%100, int(now.Month()))
	return nextNumeric(db, prefix, 4)
}

func nextNumeric(db *gorm.DB, prefix string, digits int) (string, error) {
	var codes []string
	if err := db.Table("quotations").Where("code LIKE ?", prefix+"%").Pluck("code", &codes).Error; err != nil {
		return "", err
	}

	maxSequence := 0
	for _, code := range codes {
		sequence, err := strconv.Atoi(strings.TrimPrefix(code, prefix))
		if err == nil && sequence > maxSequence {
			maxSequence = sequence
		}
	}
	return fmt.Sprintf("%s%0*d", prefix, digits, maxSequence+1), nil
}
