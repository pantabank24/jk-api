package usecase

import (
	"errors"
	"strings"

	"jk-api/internal/entity"
	"jk-api/internal/module/quotation_intake/repository"
)

// IntakeRequest is the whole form on หน้าเปิดใบเสนอราคา: who the goods came from
// and any note. The photos are uploaded separately (multipart) once the row
// exists, since they need an id to file themselves under.
type IntakeRequest struct {
	// Store/Branch/CreatedBy are derived from the JWT in the controller, never
	// from the payload — the same rule the quotation module follows.
	StoreID   *uint `json:"-"`
	BranchID  *uint `json:"-"`
	CreatedBy *uint `json:"-"`
	// PayloadStoreID/PayloadBranchID let a master (who belongs to no store) file
	// the intake under a chosen counter. Ignored for every other role.
	PayloadStoreID  *uint `json:"store_id"`
	PayloadBranchID *uint `json:"branch_id"`

	CustomerID    *uint  `json:"customer_id"`
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	Note          string `json:"note"`
}

type IntakeUsecase interface {
	GetAll(f repository.IntakeFilter, page, limit int) ([]entity.QuotationIntake, int64, error)
	GetByID(id uint) (*entity.QuotationIntake, error)
	Create(req *IntakeRequest) (*entity.QuotationIntake, error)
	Update(id uint, req *IntakeRequest) (*entity.QuotationIntake, error)
	Cancel(id uint) (*entity.QuotationIntake, error)
	Delete(id uint) error

	AddImages(intakeID uint, urls []string, imageType string) error
	DeleteImage(intakeID, imageID uint) (string, error)
}

type intakeUsecase struct {
	repo repository.IntakeRepository
}

func NewIntakeUsecase(repo repository.IntakeRepository) IntakeUsecase {
	return &intakeUsecase{repo: repo}
}

func (u *intakeUsecase) GetAll(f repository.IntakeFilter, page, limit int) ([]entity.QuotationIntake, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 20
	}
	return u.repo.FindAll(f, page, limit)
}

func (u *intakeUsecase) GetByID(id uint) (*entity.QuotationIntake, error) {
	return u.repo.FindByID(id)
}

func (u *intakeUsecase) Create(req *IntakeRequest) (*entity.QuotationIntake, error) {
	name := strings.TrimSpace(req.CustomerName)
	if name == "" {
		return nil, errors.New("กรุณากรอกชื่อลูกค้า")
	}
	intake := &entity.QuotationIntake{
		StoreID:       req.StoreID,
		BranchID:      req.BranchID,
		CreatedBy:     req.CreatedBy,
		CustomerID:    req.CustomerID,
		CustomerName:  name,
		CustomerPhone: strings.TrimSpace(req.CustomerPhone),
		Note:          req.Note,
		Status:        entity.IntakeStatusOpen,
	}
	if err := u.repo.Create(intake); err != nil {
		return nil, err
	}
	return u.repo.FindByID(intake.ID)
}

func (u *intakeUsecase) Update(id uint, req *IntakeRequest) (*entity.QuotationIntake, error) {
	intake, err := u.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("ไม่พบใบเปิดงาน")
	}
	// A used intake is a record of what was handed over at the counter and what
	// the quotation was issued against — editing it after the fact would silently
	// rewrite that history.
	if intake.Status != entity.IntakeStatusOpen {
		return nil, errors.New("ใบเปิดงานนี้ถูกใช้หรือยกเลิกไปแล้ว แก้ไขไม่ได้")
	}
	name := strings.TrimSpace(req.CustomerName)
	if name == "" {
		return nil, errors.New("กรุณากรอกชื่อลูกค้า")
	}
	intake.CustomerID = req.CustomerID
	intake.CustomerName = name
	intake.CustomerPhone = strings.TrimSpace(req.CustomerPhone)
	intake.Note = req.Note
	if err := u.repo.Save(intake); err != nil {
		return nil, err
	}
	return u.repo.FindByID(intake.ID)
}

func (u *intakeUsecase) Cancel(id uint) (*entity.QuotationIntake, error) {
	intake, err := u.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("ไม่พบใบเปิดงาน")
	}
	if intake.Status == entity.IntakeStatusUsed {
		return nil, errors.New("ใบเปิดงานนี้ออกใบเสนอราคาไปแล้ว ยกเลิกไม่ได้")
	}
	intake.Status = entity.IntakeStatusCancelled
	if err := u.repo.Save(intake); err != nil {
		return nil, err
	}
	return u.repo.FindByID(intake.ID)
}

func (u *intakeUsecase) Delete(id uint) error {
	intake, err := u.repo.FindByID(id)
	if err != nil {
		return errors.New("ไม่พบใบเปิดงาน")
	}
	if intake.Status == entity.IntakeStatusUsed {
		return errors.New("ใบเปิดงานนี้ออกใบเสนอราคาไปแล้ว ลบไม่ได้")
	}
	return u.repo.Delete(id)
}

func (u *intakeUsecase) AddImages(intakeID uint, urls []string, imageType string) error {
	if imageType != entity.IntakeImageBeforeMelt && imageType != entity.IntakeImageIDCard {
		return errors.New("ชนิดรูปไม่ถูกต้อง")
	}
	intake, err := u.repo.FindByID(intakeID)
	if err != nil {
		return errors.New("ไม่พบใบเปิดงาน")
	}
	if intake.Status != entity.IntakeStatusOpen {
		return errors.New("ใบเปิดงานนี้ถูกใช้หรือยกเลิกไปแล้ว เพิ่มรูปไม่ได้")
	}
	return u.repo.AddImages(intakeID, urls, imageType)
}

// DeleteImage removes one photo and returns its stored path so the caller can
// drop the file too — an unreferenced upload left on disk is a photo of someone's
// ID card that nothing in the app can ever show or delete again.
func (u *intakeUsecase) DeleteImage(intakeID, imageID uint) (string, error) {
	intake, err := u.repo.FindByID(intakeID)
	if err != nil {
		return "", errors.New("ไม่พบใบเปิดงาน")
	}
	if intake.Status != entity.IntakeStatusOpen {
		return "", errors.New("ใบเปิดงานนี้ถูกใช้หรือยกเลิกไปแล้ว ลบรูปไม่ได้")
	}
	img, err := u.repo.FindImage(intakeID, imageID)
	if err != nil {
		return "", errors.New("ไม่พบรูปนี้")
	}
	if err := u.repo.DeleteImage(img.ID); err != nil {
		return "", err
	}
	return img.ImageURL, nil
}
