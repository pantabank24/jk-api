package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"jk-api/internal/entity"
	"jk-api/internal/middleware"
	"jk-api/internal/module/quotation_intake/repository"
	"jk-api/internal/module/quotation_intake/usecase"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type IntakeController struct {
	uc usecase.IntakeUsecase
	db *gorm.DB
}

func NewIntakeController(uc usecase.IntakeUsecase, db *gorm.DB) *IntakeController {
	return &IntakeController{uc: uc, db: db}
}

// scopeOf resolves which intakes the caller may see, from their token alone.
// Deliberately NOT scoped to created_by: the whole point of an intake is that
// the goods come in on one shift and the quotation goes out on another, so
// whoever is on the counter must be able to pick up a colleague's open job.
func scopeOf(c *fiber.Ctx) (storeID *uint, branchID *uint) {
	if middleware.IsMaster(c) {
		// master has no store of their own — they may narrow with query params.
		if v := c.Query("store_id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				uid := uint(id)
				storeID = &uid
			}
		}
		if v := c.Query("branch_id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				uid := uint(id)
				branchID = &uid
			}
		}
		return storeID, branchID
	}
	storeID = middleware.GetStoreID(c)
	// owner sees every branch of their store; employee is pinned to their branch.
	if middleware.GetRoleName(c) != "owner" {
		branchID = middleware.GetBranchID(c)
	}
	return storeID, branchID
}

// inScope guards the single-row routes: an id in the URL must not reach across
// stores just because the caller holds quotations.read somewhere else.
func inScope(c *fiber.Ctx, intake *entity.QuotationIntake) bool {
	if middleware.IsMaster(c) {
		return true
	}
	storeID := middleware.GetStoreID(c)
	if storeID != nil && (intake.StoreID == nil || *intake.StoreID != *storeID) {
		return false
	}
	if middleware.GetRoleName(c) == "owner" {
		return true
	}
	branchID := middleware.GetBranchID(c)
	if branchID != nil && (intake.BranchID == nil || *intake.BranchID != *branchID) {
		return false
	}
	return true
}

// load fetches the intake named by :id and checks it is in the caller's scope.
func (ctrl *IntakeController) load(c *fiber.Ctx) (*entity.QuotationIntake, error) {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return nil, response.BadRequest(c, "Invalid intake ID")
	}
	intake, err := ctrl.uc.GetByID(uint(id))
	if err != nil {
		return nil, response.NotFound(c, "ไม่พบใบเปิดงาน")
	}
	if !inScope(c, intake) {
		return nil, response.NotFound(c, "ไม่พบใบเปิดงาน")
	}
	return intake, nil
}

func (ctrl *IntakeController) GetAll(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	storeID, branchID := scopeOf(c)
	f := repository.IntakeFilter{
		StoreID:  storeID,
		BranchID: branchID,
		Search:   c.Query("search"),
	}
	if s := c.Query("status"); s != "" {
		if st, err := strconv.Atoi(s); err == nil {
			f.Status = &st
		}
	}

	intakes, total, err := ctrl.uc.GetAll(f, page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Intakes retrieved", intakes, page, limit, total)
}

func (ctrl *IntakeController) GetByID(c *fiber.Ctx) error {
	intake, errResp := ctrl.load(c)
	if errResp != nil {
		return errResp
	}
	return response.Success(c, "Intake retrieved", intake)
}

func (ctrl *IntakeController) Create(c *fiber.Ctx) error {
	var req usecase.IntakeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Store/branch are stamped from the token so the row lands in the list of the
	// counter that actually took the goods in.
	if middleware.IsMaster(c) {
		req.StoreID, req.BranchID = req.PayloadStoreID, req.PayloadBranchID
	} else {
		req.StoreID = middleware.GetStoreID(c)
		req.BranchID = middleware.GetBranchID(c)
		if req.BranchID == nil {
			// owner has no branch on their token — fall back to the store's main one
			// so their intakes are still visible to that branch's staff.
			req.BranchID = mainBranchOf(ctrl.db, req.StoreID)
		}
	}
	userID := middleware.GetUserID(c)
	req.CreatedBy = &userID

	intake, err := ctrl.uc.Create(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf(
		"เปิดใบเสนอราคา (ใบเปิดงาน #%d) ให้ %s", intake.ID, intake.CustomerName,
	))
	return response.Created(c, "Intake created", intake)
}

func mainBranchOf(db *gorm.DB, storeID *uint) *uint {
	if storeID == nil {
		return nil
	}
	var branch entity.Branch
	if err := db.Where("store_id = ? AND is_main = ?", *storeID, true).First(&branch).Error; err != nil {
		return nil
	}
	return &branch.ID
}

func (ctrl *IntakeController) Update(c *fiber.Ctx) error {
	intake, errResp := ctrl.load(c)
	if errResp != nil {
		return errResp
	}
	var req usecase.IntakeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	updated, err := ctrl.uc.Update(intake.ID, &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf("แก้ไขใบเปิดงาน #%d", intake.ID))
	return response.Success(c, "Intake updated", updated)
}

func (ctrl *IntakeController) Cancel(c *fiber.Ctx) error {
	intake, errResp := ctrl.load(c)
	if errResp != nil {
		return errResp
	}
	updated, err := ctrl.uc.Cancel(intake.ID)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	middleware.SetActivityDescription(c, fmt.Sprintf("ยกเลิกใบเปิดงาน #%d", intake.ID))
	return response.Success(c, "Intake cancelled", updated)
}

func (ctrl *IntakeController) Delete(c *fiber.Ctx) error {
	intake, errResp := ctrl.load(c)
	if errResp != nil {
		return errResp
	}
	if err := ctrl.uc.Delete(intake.ID); err != nil {
		return response.BadRequest(c, err.Error())
	}
	// Drop the whole folder with it — the photos have no other owner, and one of
	// them is an ID card.
	_ = os.RemoveAll(intakeDir(intake.ID))
	middleware.SetActivityDescription(c, fmt.Sprintf("ลบใบเปิดงาน #%d", intake.ID))
	return response.Success(c, "Intake deleted", nil)
}

func intakeDir(id uint) string {
	return fmt.Sprintf("./uploads/quotation-intakes/%d", id)
}

func (ctrl *IntakeController) UploadImages(c *fiber.Ctx) error {
	intake, errResp := ctrl.load(c)
	if errResp != nil {
		return errResp
	}

	form, err := c.MultipartForm()
	if err != nil {
		return response.BadRequest(c, "Invalid form data")
	}
	files := form.File["images"]
	if len(files) == 0 {
		return response.BadRequest(c, "No images provided")
	}

	imageType := c.FormValue("type")
	if imageType == "" {
		imageType = c.Query("type")
	}

	dir := intakeDir(intake.ID)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return response.InternalServerError(c, "Failed to create directory")
	}

	var urls []string
	for _, file := range files {
		ext := filepath.Ext(file.Filename)
		filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
		if err := c.SaveFile(file, fmt.Sprintf("%s/%s", dir, filename)); err != nil {
			return response.InternalServerError(c, "Failed to save file")
		}
		urls = append(urls, fmt.Sprintf("/uploads/quotation-intakes/%d/%s", intake.ID, filename))
	}

	if err := ctrl.uc.AddImages(intake.ID, urls, imageType); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Images uploaded", urls)
}

func (ctrl *IntakeController) DeleteImage(c *fiber.Ctx) error {
	intake, errResp := ctrl.load(c)
	if errResp != nil {
		return errResp
	}
	imageID, err := strconv.ParseUint(c.Params("imageId"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid image ID")
	}
	url, err := ctrl.uc.DeleteImage(intake.ID, uint(imageID))
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	// url is "/uploads/…" as served; the file sits under ./uploads.
	if strings.HasPrefix(url, "/uploads/") {
		_ = os.Remove("." + url)
	}
	return response.Success(c, "Image deleted", nil)
}
