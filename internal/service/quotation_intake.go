package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jk-api/internal/entity"

	"gorm.io/gorm"
)

// ConsumeIntake closes an ใบเปิดงาน against the quotation just issued from it:
// the photos taken at the counter (before-melt, ID card) are copied onto the
// quotation, and the intake is stamped used so it drops off the open list.
//
// The files are copied rather than referenced, so the quotation owns a complete
// set of its own images: deleting the intake later (which takes its folder with
// it) can never blank out a document that has already been issued and printed.
//
// Best-effort by design — the quotation is already saved by the time this runs,
// and failing to move a photo must not fail the sale. The error is returned so
// the caller can log it, not so it can roll anything back.
func ConsumeIntake(db *gorm.DB, intakeID, quotationID uint) error {
	var intake entity.QuotationIntake
	if err := db.Preload("Images").First(&intake, intakeID).Error; err != nil {
		return fmt.Errorf("intake %d not found: %w", intakeID, err)
	}
	// Already spent on another quotation — never re-attach its photos to a second
	// document, or two quotations would claim the same goods.
	if intake.Status != entity.IntakeStatusOpen {
		return fmt.Errorf("intake %d is not open", intakeID)
	}

	dir := fmt.Sprintf("./uploads/quotations/%d", quotationID)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("create quotation dir: %w", err)
	}

	var copied []entity.QuotationImage
	for _, img := range intake.Images {
		if !strings.HasPrefix(img.ImageURL, "/uploads/") {
			continue
		}
		ext := filepath.Ext(img.ImageURL)
		name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
		if err := copyFile("."+img.ImageURL, filepath.Join(dir, name)); err != nil {
			// One unreadable photo should not cost the quotation the rest of them.
			continue
		}
		copied = append(copied, entity.QuotationImage{
			QuotationID: quotationID,
			ImageURL:    fmt.Sprintf("/uploads/quotations/%d/%s", quotationID, name),
			Type:        img.Type,
		})
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if len(copied) > 0 {
			if err := tx.Create(&copied).Error; err != nil {
				return err
			}
		}
		now := time.Now()
		return tx.Model(&entity.QuotationIntake{}).
			Where("id = ? AND status = ?", intakeID, entity.IntakeStatusOpen).
			Updates(map[string]any{
				"status":       entity.IntakeStatusUsed,
				"quotation_id": quotationID,
				"used_at":      now,
			}).Error
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
