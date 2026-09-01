package repository

import (
	"jk-api/internal/entity"
	"jk-api/internal/verification"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AuthRepository interface {
	FindByEmail(email string) (*entity.User, error)
	FindByIDWithRole(id uint) (*entity.User, error)
	GetPermissionsByRoleID(roleID uint) ([]string, error)
	GetMemberCreditsByUserID(userID uint) (float64, bool)
	EmailExistsForOtherUser(email string, excludeID uint) (bool, error)
	UpdateProfile(userID uint, fields map[string]interface{}) error
	GetConfigValue(key string) string
	HasConsent(userID uint, kind string, version int) bool
	CreateAcknowledgement(consent *entity.UserConsent) error
	AppendConsent(consent *entity.UserConsent) error
	LatestConsent(userID uint, kind string) (*entity.UserConsent, bool)
}

type authRepository struct {
	db *gorm.DB
}

func NewAuthRepository(db *gorm.DB) AuthRepository {
	return &authRepository{db: db}
}

func (r *authRepository) FindByEmail(email string) (*entity.User, error) {
	var user entity.User
	err := r.db.Preload("Role").Preload("Store").Preload("Branch").
		Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *authRepository) FindByIDWithRole(id uint) (*entity.User, error) {
	var user entity.User
	err := r.db.Preload("Role").Preload("Store").Preload("Branch").Preload("Bank").
		First(&user, id).Error
	if err != nil {
		return nil, err
	}
	// Feeds the verify badge on the customer's own home and โปรไฟล์ของฉัน screens.
	// Staff have no high-priority documents, so they come back as "none".
	user.VerificationStatus = verification.StatusOf(r.db, user.ID)
	return &user, nil
}

// GetMemberCreditsByUserID returns the credit balance of the member profile
// linked to the user, and whether such a profile exists.
func (r *authRepository) GetMemberCreditsByUserID(userID uint) (float64, bool) {
	var member entity.Member
	err := r.db.Select("credits").Where("user_id = ?", userID).First(&member).Error
	if err != nil {
		return 0, false
	}
	return member.Credits, true
}

// EmailExistsForOtherUser reports whether the email is already used by a
// user other than excludeID (used to guard self-service profile updates).
func (r *authRepository) EmailExistsForOtherUser(email string, excludeID uint) (bool, error) {
	var count int64
	err := r.db.Model(&entity.User{}).
		Where("email = ? AND id <> ?", email, excludeID).
		Count(&count).Error
	return count > 0, err
}

// UpdateProfile applies a partial column update to the user, leaving
// association / FK columns untouched.
func (r *authRepository) UpdateProfile(userID uint, fields map[string]interface{}) error {
	return r.db.Model(&entity.User{}).
		Where("id = ?", userID).
		Updates(fields).Error
}

func (r *authRepository) GetPermissionsByRoleID(roleID uint) ([]string, error) {
	var permissions []string
	err := r.db.Model(&entity.RolePermission{}).
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_id = ?", roleID).
		Pluck("permissions.code", &permissions).Error
	return permissions, err
}

// GetConfigValue reads one system_configs row. The PDPA text and its version are
// shop settings, not auth data, but resolving "does this user still need to
// consent?" needs both halves in one place — and going through the config module
// would make the auth package depend on it for two string lookups.
func (r *authRepository) GetConfigValue(key string) string {
	var cfg entity.SystemConfig
	if err := r.db.Where("key = ?", key).First(&cfg).Error; err != nil {
		return ""
	}
	return cfg.Value
}

// HasConsent answers whether the user has already accepted this exact version.
// A newer version means the row is absent and the modal comes back, which is the
// whole mechanism behind "แก้ข้อความแล้วต้องยอมรับใหม่".
func (r *authRepository) HasConsent(userID uint, kind string, version int) bool {
	var count int64
	err := r.db.Model(&entity.UserConsent{}).
		Where("user_id = ? AND consent_type = ? AND version = ?", userID, kind, version).
		Count(&count).Error
	return err == nil && count > 0
}

// CreateAcknowledgement records that the user read one version of the notice.
// A double tap — or a second tab left open on the same modal — collapses into
// the row that is already there rather than filing a second piece of evidence
// for the same act. The conflict target repeats the partial index's WHERE from
// migration 000098; Postgres will not match a partial index without it.
func (r *authRepository) CreateAcknowledgement(consent *entity.UserConsent) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"}, {Name: "consent_type"}, {Name: "version"},
		},
		TargetWhere: clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: "consent_type", Value: "pdpa"},
		}},
		DoNothing: true,
	}).Create(consent).Error
}

// AppendConsent adds a row to the consent log without collapsing anything —
// used by the withdrawable consents, where "granted, then withdrawn, then
// granted again" is three facts and not one.
func (r *authRepository) AppendConsent(consent *entity.UserConsent) error {
	return r.db.Create(consent).Error
}

// LatestConsent returns the most recent row for one consent type, which is what
// the current state is: the newest row's Granted. ok is false when the user has
// never answered either way.
func (r *authRepository) LatestConsent(userID uint, kind string) (*entity.UserConsent, bool) {
	var row entity.UserConsent
	err := r.db.Where("user_id = ? AND consent_type = ?", userID, kind).
		Order("id DESC").First(&row).Error
	if err != nil {
		return nil, false
	}
	return &row, true
}
