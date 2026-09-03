package repository

import (
	"errors"
	"sync"

	"jk-api/internal/entity"
	"jk-api/internal/verification"

	"gorm.io/gorm"
)

// CustomerRepository manages customer accounts. A customer is a User whose role
// is "customer"; every list/lookup here is scoped to that role.
type CustomerRepository interface {
	GetCustomerRoleID() (uint, error)
	Create(user *entity.User) error
	FindAll(page, limit int, storeID *uint, search string) ([]entity.User, int64, error)
	// FindSoleStoreID returns the shop's only store, or nil when there is more than
	// one. Lets a single-store shop create customers without having to name a store.
	FindSoleStoreID() (*uint, error)
	FindByID(id uint) (*entity.User, error)
	Update(user *entity.User) error
	Delete(id uint) error
	ExistsByEmail(email string) bool

	// Documents uploaded for a customer.
	CreateDocument(doc *entity.CustomerDocument) error
	FindDocuments(userID uint) ([]entity.CustomerDocument, error)
	FindDocumentByID(id uint) (*entity.CustomerDocument, error)
	UpdateDocument(doc *entity.CustomerDocument) error
	DeleteDocument(id uint) error
	// FindActiveDocumentType looks up an enabled type from the master list, so an
	// upload can be refused with a clear message instead of tripping the foreign
	// key — and so the caller can see whether it is high priority.
	FindActiveDocumentType(id uint) (*entity.DocumentType, error)
	// FindDocumentsOfType returns a customer's existing documents of one type. A
	// high-priority type holds a single document, so re-uploading replaces what is
	// there rather than stacking another copy on top.
	FindDocumentsOfType(userID, typeID uint) ([]entity.CustomerDocument, error)
	// FindReviewerIDs lists the staff who should hear about a document needing
	// review: every master (they oversee all stores) plus the owners and employees
	// of the customer's own store.
	FindReviewerIDs(storeID *uint) ([]uint, error)
}

// thaiCollation คือ ICU collation ที่เรียงภาษาไทยตามลำดับพจนานุกรม
const thaiCollation = "th-TH-x-icu"

type customerRepository struct {
	db *gorm.DB

	// ORDER BY สำหรับลิสต์ลูกค้า — ตรวจครั้งเดียวว่า DB มี ICU collation ไหม
	nameOrderOnce sync.Once
	nameOrderSQL  string
}

// nameOrder returns the ORDER BY that lists customers ตามพยัญชนะชื่อ.
//
// การเรียงตามค่า byte ล้วน ๆ ได้ลำดับพยัญชนะถูก (ก–ฮ คือ U+0E01–U+0E2E เรียงตาม
// ตัวอักษรอยู่แล้ว) แต่ผิดกับชื่อที่ขึ้นต้นด้วยสระหน้า: เ แ โ ใ ไ อยู่ที่ U+0E40–U+0E44
// ซึ่งมาหลัง ฮ ทั้งหมด "เกียรติ" จึงไปตกท้ายลิสต์แทนที่จะอยู่ใต้ ก. ICU collation
// จัดการเรื่องนี้ให้.
//
// postgres:16 ที่ระบบนี้ใช้มี ICU มาให้อยู่แล้ว แต่ถ้าเจอ DB ที่ build มาโดยไม่มี
// ICU ให้ตกกลับไปใช้ collation เริ่มต้น — ลิสต์เรียงไม่สวยดีกว่าลิสต์พังทั้งหน้า
func (r *customerRepository) nameOrder() string {
	r.nameOrderOnce.Do(func() {
		r.nameOrderSQL = "users.name ASC, users.id ASC"
		var n int64
		if err := r.db.Raw("SELECT count(*) FROM pg_collation WHERE collname = ?",
			thaiCollation).Scan(&n).Error; err == nil && n > 0 {
			r.nameOrderSQL = `users.name COLLATE "` + thaiCollation + `" ASC, users.id ASC`
		}
	})
	return r.nameOrderSQL
}

func NewCustomerRepository(db *gorm.DB) CustomerRepository {
	return &customerRepository{db: db}
}

func (r *customerRepository) GetCustomerRoleID() (uint, error) {
	var role entity.Role
	if err := r.db.Where("name = ?", "customer").First(&role).Error; err != nil {
		return 0, errors.New("customer role not found")
	}
	return role.ID, nil
}

func (r *customerRepository) Create(user *entity.User) error {
	return r.db.Create(user).Error
}

func (r *customerRepository) FindAll(page, limit int, storeID *uint, search string) ([]entity.User, int64, error) {
	var users []entity.User
	var total int64

	roleID, err := r.GetCustomerRoleID()
	if err != nil {
		return nil, 0, err
	}

	// กรองเฉพาะร้าน ไม่กรองสาขา — ลูกค้าเป็นข้อมูลระดับร้าน ทุกสาขาในร้านเดียวกัน
	// เห็นชุดเดียวกัน (ดูคอมเมนต์ที่ controller)
	query := r.db.Model(&entity.User{}).Where("role_id = ?", roleID)
	if storeID != nil {
		query = query.Where("store_id = ?", *storeID)
	}
	if search != "" {
		query = query.Where("name ILIKE ? OR email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	query.Count(&total)
	offset := (page - 1) * limit
	err = query.Preload("Role").Preload("Store").Preload("Branch").
		Offset(offset).Limit(limit).Order(r.nameOrder()).Find(&users).Error
	if err == nil {
		// One extra query for the whole page — the list draws a verify badge per row.
		verification.Apply(r.db, users)
	}
	return users, total, err
}

func (r *customerRepository) FindSoleStoreID() (*uint, error) {
	var ids []uint
	// Two is enough to know it is not "sole" — no need to count the whole table.
	if err := r.db.Model(&entity.Store{}).Order("id").Limit(2).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) != 1 {
		return nil, nil
	}
	return &ids[0], nil
}

func (r *customerRepository) FindByID(id uint) (*entity.User, error) {
	roleID, err := r.GetCustomerRoleID()
	if err != nil {
		return nil, err
	}
	var user entity.User
	err = r.db.Preload("Role").Preload("Store").Preload("Branch").Preload("Bank").
		Where("role_id = ?", roleID).First(&user, id).Error
	if err != nil {
		return nil, err
	}
	user.VerificationStatus = verification.StatusOf(r.db, user.ID)
	return &user, nil
}

func (r *customerRepository) Update(user *entity.User) error {
	return r.db.Save(user).Error
}

func (r *customerRepository) Delete(id uint) error {
	roleID, err := r.GetCustomerRoleID()
	if err != nil {
		return err
	}
	return r.db.Where("role_id = ?", roleID).Delete(&entity.User{}, id).Error
}

func (r *customerRepository) ExistsByEmail(email string) bool {
	var count int64
	r.db.Model(&entity.User{}).Where("email = ?", email).Count(&count)
	return count > 0
}

func (r *customerRepository) CreateDocument(doc *entity.CustomerDocument) error {
	return r.db.Create(doc).Error
}

func (r *customerRepository) FindDocuments(userID uint) ([]entity.CustomerDocument, error) {
	var docs []entity.CustomerDocument
	// Preload the type so the list can show its label without a second round trip.
	err := r.db.Preload("DocumentType").Where("user_id = ?", userID).Order("id DESC").Find(&docs).Error
	return docs, err
}

func (r *customerRepository) FindDocumentByID(id uint) (*entity.CustomerDocument, error) {
	var doc entity.CustomerDocument
	// Preloaded because callers need IsHighPriority to decide whether the document
	// may be deleted, and whether it is reviewable at all.
	if err := r.db.Preload("DocumentType").First(&doc, id).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *customerRepository) UpdateDocument(doc *entity.CustomerDocument) error {
	return r.db.Save(doc).Error
}

func (r *customerRepository) DeleteDocument(id uint) error {
	return r.db.Delete(&entity.CustomerDocument{}, id).Error
}

func (r *customerRepository) FindActiveDocumentType(id uint) (*entity.DocumentType, error) {
	var t entity.DocumentType
	if err := r.db.Where("is_active = ?", true).First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *customerRepository) FindDocumentsOfType(userID, typeID uint) ([]entity.CustomerDocument, error) {
	var docs []entity.CustomerDocument
	err := r.db.Where("user_id = ? AND document_type_id = ?", userID, typeID).Find(&docs).Error
	return docs, err
}

func (r *customerRepository) FindReviewerIDs(storeID *uint) ([]uint, error) {
	var ids []uint
	q := r.db.Model(&entity.User{}).
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("users.is_active = ?", true)
	if storeID == nil {
		// Customer belongs to no store yet — only master can be sure to reach it.
		q = q.Where("roles.name = ?", "master")
	} else {
		// Parenthesised explicitly so the OR cannot escape the is_active filter.
		q = q.Where("(roles.name = ? OR (roles.name IN ? AND users.store_id = ?))",
			"master", []string{"owner", "employee"}, *storeID)
	}
	err := q.Pluck("users.id", &ids).Error
	return ids, err
}
