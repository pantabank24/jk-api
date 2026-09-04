package repository

import (
	"jk-api/internal/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// IntakeFilter scopes the list page. StoreID/BranchID come from the caller's
// token (never from the query string, except for a master who has no store of
// their own) so a branch can only ever see its own counter's open jobs.
type IntakeFilter struct {
	StoreID  *uint
	BranchID *uint
	// Status is the tab: nil = ทั้งหมด.
	Status *int
	// Search matches the typed customer name or phone — one box on the list page.
	Search string
}

type IntakeRepository interface {
	FindAll(f IntakeFilter, page, limit int) ([]entity.QuotationIntake, int64, error)
	FindByID(id uint) (*entity.QuotationIntake, error)
	Create(intake *entity.QuotationIntake) error
	Save(intake *entity.QuotationIntake) error
	Delete(id uint) error

	AddImages(intakeID uint, urls []string, imageType string) error
	FindImage(intakeID, imageID uint) (*entity.QuotationIntakeImage, error)
	DeleteImage(id uint) error
}

type intakeRepository struct {
	db *gorm.DB
}

func NewIntakeRepository(db *gorm.DB) IntakeRepository {
	return &intakeRepository{db: db}
}

func (r *intakeRepository) scope(f IntakeFilter) *gorm.DB {
	q := r.db.Model(&entity.QuotationIntake{})
	if f.StoreID != nil {
		q = q.Where("store_id = ?", *f.StoreID)
	}
	if f.BranchID != nil {
		q = q.Where("branch_id = ?", *f.BranchID)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	if f.Search != "" {
		like := "%" + f.Search + "%"
		q = q.Where("customer_name ILIKE ? OR customer_phone ILIKE ?", like, like)
	}
	return q
}

func (r *intakeRepository) FindAll(f IntakeFilter, page, limit int) ([]entity.QuotationIntake, int64, error) {
	var intakes []entity.QuotationIntake
	var total int64

	q := r.scope(f)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	// Newest first: the counter works off the top of this list.
	err := r.scope(f).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Preload("Creator").Preload("Customer").Preload("Branch").
		Offset((page - 1) * limit).Limit(limit).
		Order("id DESC").Find(&intakes).Error
	return intakes, total, err
}

func (r *intakeRepository) FindByID(id uint) (*entity.QuotationIntake, error) {
	var intake entity.QuotationIntake
	err := r.db.
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Preload("Creator").Preload("Customer").Preload("Customer.Bank").
		Preload("Store").Preload("Branch").
		First(&intake, id).Error
	if err != nil {
		return nil, err
	}
	return &intake, nil
}

func (r *intakeRepository) Create(intake *entity.QuotationIntake) error {
	return r.db.Create(intake).Error
}

func (r *intakeRepository) Save(intake *entity.QuotationIntake) error {
	// Omit associations: the row handed in was loaded with its Images preloaded,
	// and a full-graph Save would upsert every one of them on what is only ever a
	// field edit. Photos are added and removed through their own methods.
	return r.db.Omit(clause.Associations).Save(intake).Error
}

func (r *intakeRepository) Delete(id uint) error {
	return r.db.Delete(&entity.QuotationIntake{}, id).Error
}

func (r *intakeRepository) AddImages(intakeID uint, urls []string, imageType string) error {
	if len(urls) == 0 {
		return nil
	}
	images := make([]entity.QuotationIntakeImage, 0, len(urls))
	for _, url := range urls {
		images = append(images, entity.QuotationIntakeImage{
			IntakeID: intakeID,
			ImageURL: url,
			Type:     imageType,
		})
	}
	return r.db.Create(&images).Error
}

func (r *intakeRepository) FindImage(intakeID, imageID uint) (*entity.QuotationIntakeImage, error) {
	var img entity.QuotationIntakeImage
	// Scoped by intake as well as id: the caller's right to touch the image comes
	// from their right to the intake it hangs off.
	err := r.db.Where("intake_id = ?", intakeID).First(&img, imageID).Error
	if err != nil {
		return nil, err
	}
	return &img, nil
}

func (r *intakeRepository) DeleteImage(id uint) error {
	return r.db.Delete(&entity.QuotationIntakeImage{}, id).Error
}
