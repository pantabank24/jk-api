package usecase

import (
	"errors"
	"fmt"

	"jk-api/internal/entity"
	"jk-api/internal/module/member/repository"
	notificationRepo "jk-api/internal/module/notification/repository"
	roleRepo "jk-api/internal/module/role/repository"
)

// creditsUsePermission is the permission code whose holders have their
// quotations charged against member credits. It is the single source of truth
// for "this member is part of the credit system" — granting/revoking it via a
// migration is enough; no role names are hardcoded.
const creditsUsePermission = "credits.use"

type MemberUsecase interface {
	CreateMember(req *CreateMemberRequest) (*entity.Member, error)
	GetAllMembers(storeID *uint, branchID *uint, page, limit int, search string, status *int, memberType string) ([]entity.Member, int64, error)
	GetMemberByID(id uint) (*entity.Member, error)
	UpdateMember(id uint, req *UpdateMemberRequest) (*entity.Member, error)
	DeleteMember(id uint) error
	UpdateImage(id uint, path string) (*entity.Member, error)
	AddCredit(id uint, req *CreditRequest) (*entity.Member, error)
	GetCreditTransactions(memberID uint, page, limit int) ([]entity.CreditTransaction, int64, error)
	GetAllCreditTransactions(storeID, branchID, memberID *uint, source string, page, limit int, search string) ([]entity.CreditTransaction, int64, error)
}

type CreateMemberRequest struct {
	StoreID  *uint   `json:"store_id"`
	BranchID *uint   `json:"branch_id"`
	UserID   *uint   `json:"-"` // set by controller after creating user account
	Fname    string  `json:"fname" validate:"required"`
	Lname    string  `json:"lname" validate:"required"`
	Phone    string  `json:"phone"`
	Credits  float64 `json:"credits"`
}

type UpdateMemberRequest struct {
	Fname  string `json:"fname"`
	Lname  string `json:"lname"`
	Phone  string `json:"phone"`
	Status *int   `json:"status"`
}

type CreditRequest struct {
	Action      int     `json:"action" validate:"required"` // 0=deposit, 1=withdraw
	Amount      float64 `json:"amount" validate:"required"`
	Description string  `json:"description"`
	StoreID     *uint   `json:"store_id"`
	BranchID    *uint   `json:"branch_id"`
	CreatedBy   *uint   `json:"created_by"`
}

type memberUsecase struct {
	memberRepo repository.MemberRepository
	notifRepo  notificationRepo.NotificationRepository
	roleRepo   roleRepo.RoleRepository
}

func NewMemberUsecase(memberRepo repository.MemberRepository, notifRepo notificationRepo.NotificationRepository, roleRepo roleRepo.RoleRepository) MemberUsecase {
	return &memberUsecase{memberRepo: memberRepo, notifRepo: notifRepo, roleRepo: roleRepo}
}

// roleUsesCredits reports whether a role holds the credits.use permission.
// Results are memoized in cache so annotating a list of members costs at most
// one lookup per distinct role.
func (u *memberUsecase) roleUsesCredits(roleID uint, cache map[uint]bool) bool {
	if v, ok := cache[roleID]; ok {
		return v
	}
	perms, _ := u.roleRepo.GetPermissionsByRoleID(roleID)
	used := false
	for _, p := range perms {
		if p.Code == creditsUsePermission {
			used = true
			break
		}
	}
	cache[roleID] = used
	return used
}

// memberUsesCredits reports whether a member is subject to credit management:
// walk-in customers (no user account / no role) always are, otherwise it
// depends on whether their user's role holds credits.use.
func (u *memberUsecase) memberUsesCredits(m *entity.Member, cache map[uint]bool) bool {
	if m.User == nil || m.User.RoleID == nil {
		return true
	}
	return u.roleUsesCredits(*m.User.RoleID, cache)
}

// annotateUsesCredits fills the computed UsesCredits flag on each member.
func (u *memberUsecase) annotateUsesCredits(members []entity.Member) {
	cache := map[uint]bool{}
	for i := range members {
		members[i].UsesCredits = u.memberUsesCredits(&members[i], cache)
	}
}

func (u *memberUsecase) CreateMember(req *CreateMemberRequest) (*entity.Member, error) {
	code, err := u.memberRepo.GenerateCode()
	if err != nil {
		return nil, err
	}

	member := &entity.Member{
		StoreID:  req.StoreID,
		BranchID: req.BranchID,
		UserID:   req.UserID,
		Code:     code,
		Fname:    req.Fname,
		Lname:    req.Lname,
		Phone:    req.Phone,
		Credits:  req.Credits,
		Status:   0,
	}

	if err := u.memberRepo.Create(member); err != nil {
		return nil, err
	}

	// Record initial credit deposit transaction if credits > 0
	if req.Credits > 0 {
		_ = u.memberRepo.CreateCreditTransaction(&entity.CreditTransaction{
			MemberID:    member.ID,
			StoreID:     req.StoreID,
			BranchID:    req.BranchID,
			Action:      0, // deposit
			Amount:      req.Credits,
			Balance:     req.Credits,
			Description: "เครดิตเริ่มต้น",
			CreatedBy:   req.UserID,
		})
	}

	return member, nil
}

func (u *memberUsecase) GetAllMembers(storeID *uint, branchID *uint, page, limit int, search string, status *int, memberType string) ([]entity.Member, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	members, total, err := u.memberRepo.FindAll(storeID, branchID, page, limit, search, status, memberType)
	if err != nil {
		return nil, 0, err
	}
	u.annotateUsesCredits(members)
	return members, total, nil
}

func (u *memberUsecase) UpdateImage(id uint, path string) (*entity.Member, error) {
	return u.memberRepo.UpdateImage(id, path)
}

func (u *memberUsecase) GetMemberByID(id uint) (*entity.Member, error) {
	member, err := u.memberRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	member.UsesCredits = u.memberUsesCredits(member, map[uint]bool{})
	return member, nil
}

func (u *memberUsecase) UpdateMember(id uint, req *UpdateMemberRequest) (*entity.Member, error) {
	member, err := u.memberRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("member not found")
	}

	if req.Fname != "" {
		member.Fname = req.Fname
	}
	if req.Lname != "" {
		member.Lname = req.Lname
	}
	if req.Phone != "" {
		member.Phone = req.Phone
	}
	if req.Status != nil {
		member.Status = *req.Status
	}

	if err := u.memberRepo.Update(member); err != nil {
		return nil, err
	}
	return member, nil
}

func (u *memberUsecase) DeleteMember(id uint) error {
	_, err := u.memberRepo.FindByID(id)
	if err != nil {
		return errors.New("member not found")
	}
	return u.memberRepo.Delete(id)
}

func (u *memberUsecase) AddCredit(id uint, req *CreditRequest) (*entity.Member, error) {
	member, err := u.memberRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("member not found")
	}

	// Only allow credit management for members that are part of the credit system:
	// walk-in customers, or members whose user role holds credits.use. Roles
	// without it (e.g. master) bypass credits on quotations entirely.
	if !u.memberUsesCredits(member, map[uint]bool{}) {
		return nil, errors.New("ไม่สามารถจัดการเครดิตของสมาชิกที่ไม่ได้ใช้ระบบเครดิต")
	}

	if req.Action == 0 { // deposit
		member.Credits += req.Amount
	} else if req.Action == 1 { // withdraw
		if member.Credits < req.Amount {
			return nil, errors.New("insufficient credits")
		}
		member.Credits -= req.Amount
	} else {
		return nil, errors.New("invalid action")
	}

	// Fall back to member's own store/branch when caller (e.g. master) has none in JWT
	storeID := req.StoreID
	if storeID == nil {
		storeID = member.StoreID
	}
	branchID := req.BranchID
	if branchID == nil {
		branchID = member.BranchID
	}

	// Create transaction record
	tx := &entity.CreditTransaction{
		MemberID:    member.ID,
		StoreID:     storeID,
		BranchID:    branchID,
		Action:      req.Action,
		Amount:      req.Amount,
		Balance:     member.Credits,
		Description: req.Description,
		CreatedBy:   req.CreatedBy,
	}

	if err := u.memberRepo.CreateCreditTransaction(tx); err != nil {
		return nil, err
	}

	if err := u.memberRepo.Update(member); err != nil {
		return nil, err
	}

	// Notify the member's linked user account
	if member.UserID != nil {
		if req.Action == 0 {
			_ = u.notifRepo.Create(&entity.Notification{
				UserID: *member.UserID,
				Type:   "credit_deposit",
				Title:  "ได้รับเครดิต",
				Body:   fmt.Sprintf("ได้รับเครดิตเพิ่ม %.2f บาท คงเหลือ %.2f บาท", req.Amount, member.Credits),
			})
		} else if req.Action == 1 {
			_ = u.notifRepo.Create(&entity.Notification{
				UserID: *member.UserID,
				Type:   "credit_withdraw",
				Title:  "หักเครดิต",
				Body:   fmt.Sprintf("เครดิตถูกหัก %.2f บาท คงเหลือ %.2f บาท", req.Amount, member.Credits),
			})
		}
	}

	return member, nil
}

func (u *memberUsecase) GetCreditTransactions(memberID uint, page, limit int) ([]entity.CreditTransaction, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return u.memberRepo.GetCreditTransactions(memberID, page, limit)
}

func (u *memberUsecase) GetAllCreditTransactions(storeID, branchID, memberID *uint, source string, page, limit int, search string) ([]entity.CreditTransaction, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return u.memberRepo.GetAllCreditTransactions(storeID, branchID, memberID, source, page, limit, search)
}
