package controller

import (
	"time"

	"jk-api/internal/middleware"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type DashboardController struct {
	db *gorm.DB
}

func NewDashboardController(db *gorm.DB) *DashboardController {
	return &DashboardController{db: db}
}

type DashboardStats struct {
	MyCredits        float64 `json:"my_credits"`
	QuotationsToday  int64   `json:"quotations_today"`
	QuotationsPending int64  `json:"quotations_pending"`
	QuotationsApproved int64 `json:"quotations_approved"`
	TotalMembers     int64   `json:"total_members"`
	TotalQuotations  int64   `json:"total_quotations"`
}

func (ctrl *DashboardController) GetStats(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	storeID := middleware.GetStoreID(c)
	branchID := middleware.GetBranchID(c)
	roleName := middleware.GetRoleName(c)

	var stats DashboardStats

	// my_credits — only applies when the user has a linked member profile
	if userID != 0 {
		var credits float64
		ctrl.db.Raw("SELECT credits FROM members WHERE user_id = ? AND deleted_at IS NULL LIMIT 1", userID).Scan(&credits)
		stats.MyCredits = credits
	}

	today := time.Now().Truncate(24 * time.Hour)

	// Build scoped quotation queries
	quotationQuery := ctrl.db.Table("quotations").Where("deleted_at IS NULL")
	memberQuery := ctrl.db.Table("members").Where("deleted_at IS NULL")

	switch roleName {
	case "master":
		// no filter — sees everything
	case "owner":
		if storeID != nil {
			quotationQuery = quotationQuery.Where("store_id = ?", *storeID)
			memberQuery = memberQuery.Where("store_id = ?", *storeID)
		}
	default: // branch, employee
		if storeID != nil {
			quotationQuery = quotationQuery.Where("store_id = ?", *storeID)
			memberQuery = memberQuery.Where("store_id = ?", *storeID)
		}
		if branchID != nil {
			quotationQuery = quotationQuery.Where("branch_id = ?", *branchID)
			memberQuery = memberQuery.Where("branch_id = ?", *branchID)
		}
	}

	quotationQuery.Count(&stats.TotalQuotations)
	quotationQuery.Where("created_at >= ?", today).Count(&stats.QuotationsToday)
	quotationQuery.Where("status = ?", 0).Count(&stats.QuotationsPending)
	quotationQuery.Where("status = ?", 1).Count(&stats.QuotationsApproved)
	memberQuery.Count(&stats.TotalMembers)

	return response.Success(c, "Dashboard stats retrieved", stats)
}
