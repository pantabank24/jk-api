package usecase

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"jk-api/internal/entity"
	"jk-api/internal/module/auth/repository"
	jwtPkg "jk-api/pkg/jwt"
)

type AuthUsecase interface {
	Login(req *LoginRequest) (*LoginResponse, error)
	GetMe(userID uint) (*MeResponse, error)
	RefreshToken(userID uint) (*TokenResponse, error)
	UpdateProfile(userID uint, req *UpdateProfileRequest, allowEmailChange bool) (*MeResponse, error)
	ChangePassword(userID uint, req *ChangePasswordRequest) error
	UpdateAvatar(userID uint, path string) (*MeResponse, error)
	AcceptPDPA(userID uint, ip, userAgent string) error
	MarketingConsent(userID uint) MarketingConsentStatus
	SetMarketingConsent(userID uint, granted bool, ip, userAgent string) error
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	Token       string      `json:"token"`
	User        interface{} `json:"user"`
	Permissions []string    `json:"permissions"`
	Credits     float64     `json:"credits"`
	// Rides along on login as well as /auth/me: the client sets its user from the
	// login response directly, so leaving it off here would let a customer see one
	// screen before the consent modal appeared.
	PDPA *PDPAStatus `json:"pdpa,omitempty"`
}

type MeResponse struct {
	User        interface{} `json:"user"`
	Permissions []string    `json:"permissions"`
	Credits     float64     `json:"credits"`
	PDPA        *PDPAStatus `json:"pdpa,omitempty"`
}

// PDPAStatus is sent only while an acknowledgement is actually outstanding —
// absent means nothing to ask, so the client never has to reason about a false
// Required.
type PDPAStatus struct {
	Required bool   `json:"required"`
	Version  int    `json:"version"`
	Text     string `json:"text"`
}

// MarketingConsentStatus drives the toggle on the customer's profile, which is
// how a consent given inside the notice modal can be taken back. Available is
// false when the shop has not published a marketing consent at all.
type MarketingConsentStatus struct {
	Available bool   `json:"available"`
	Granted   bool   `json:"granted"`
	Text      string `json:"text,omitempty"`
}

type TokenResponse struct {
	Token string `json:"token"`
}

type UpdateProfileRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	// The rest is what a customer keeps about themselves — the same fields the
	// shop edits on PUT /customers/:id. They print on the receipt (address, tax
	// id) and are paid out to (bank), so a customer maintaining their own record
	// is the point. Pointers: a client that doesn't manage a field (the staff
	// profile form) simply omits it and the stored value is left alone.
	StoreName *string `json:"store_name"`
	Address   *string `json:"address"`
	TaxID     *string `json:"tax_id"`
	// BankID uses 0 for "ไม่ระบุ" and is normalised to NULL, same as customers.
	BankID          *uint   `json:"bank_id"`
	BankAccountNo   *string `json:"bank_account_no"`
	BankAccountName *string `json:"bank_account_name"`
}

// normalizeBankID maps "no bank" (0, what an empty <Select> submits) to a NULL
// bank_id so it is never written as a real foreign key. Mirrors the customer
// module's rule of the same name.
func normalizeBankID(id *uint) *uint {
	if id == nil || *id == 0 {
		return nil
	}
	return id
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type authUsecase struct {
	authRepo  repository.AuthRepository
	jwtSecret string
	jwtExpiry time.Duration
}

func NewAuthUsecase(authRepo repository.AuthRepository, jwtSecret string, jwtExpiry time.Duration) AuthUsecase {
	return &authUsecase{
		authRepo:  authRepo,
		jwtSecret: jwtSecret,
		jwtExpiry: jwtExpiry,
	}
}

func (u *authUsecase) Login(req *LoginRequest) (*LoginResponse, error) {
	user, err := u.authRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	if !user.IsActive {
		return nil, errors.New("account is disabled")
	}

	if !jwtPkg.CheckPassword(user.Password, req.Password) {
		return nil, errors.New("invalid email or password")
	}

	// Get role name
	roleName := ""
	var roleID uint
	if user.Role != nil {
		roleName = user.Role.Name
		roleID = user.Role.ID
	}

	// Generate JWT
	claims := &jwtPkg.Claims{
		UserID:   user.ID,
		StoreID:  user.StoreID,
		BranchID: user.BranchID,
		RoleID:   roleID,
		RoleName: roleName,
	}

	token, err := jwtPkg.GenerateToken(u.jwtSecret, u.jwtExpiry, claims)
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	// Get permissions
	permissions := []string{}
	if roleID > 0 {
		permissions, _ = u.authRepo.GetPermissionsByRoleID(roleID)
	}

	credits, _ := u.authRepo.GetMemberCreditsByUserID(user.ID)

	// The user in the response has to be the same shape /auth/me returns, because
	// the client sets its user straight from here and only reaches /auth/me on a
	// reload. FindByEmail is the login lookup: it neither computes the verify
	// badge nor preloads Bank, so a verified customer would sit on a grey "ยังไม่มี
	// เอกสารยืนยันตัวตน" badge — and a blank payout bank — for the whole session
	// until something happened to refresh them. Re-read through the same call
	// /auth/me uses instead of teaching two lookups to stay in step.
	full, err := u.authRepo.FindByIDWithRole(user.ID)
	if err != nil {
		full = user
	}

	return &LoginResponse{
		Token:       token,
		User:        full,
		Permissions: permissions,
		Credits:     credits,
		PDPA:        u.pdpaStatusFor(full),
	}, nil
}

func (u *authUsecase) GetMe(userID uint) (*MeResponse, error) {
	user, err := u.authRepo.FindByIDWithRole(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	permissions := []string{}
	if user.RoleID != nil {
		permissions, _ = u.authRepo.GetPermissionsByRoleID(*user.RoleID)
	}

	credits, _ := u.authRepo.GetMemberCreditsByUserID(user.ID)

	return &MeResponse{
		User:        user,
		Permissions: permissions,
		Credits:     credits,
		PDPA:        u.pdpaStatusFor(user),
	}, nil
}

// UpdateProfile saves a user's own record. allowEmailChange is false for
// customers: their email is the login the shop issued them, so it is theirs to
// read but not to change — they ask the shop instead. Enforced here and not only
// by disabling the field, since a disabled input is no barrier to a direct call.
func (u *authUsecase) UpdateProfile(userID uint, req *UpdateProfileRequest, allowEmailChange bool) (*MeResponse, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("name is required")
	}

	fields := map[string]interface{}{
		"name":  strings.TrimSpace(req.Name),
		"phone": req.Phone,
	}

	if allowEmailChange {
		if strings.TrimSpace(req.Email) == "" {
			return nil, errors.New("email is required")
		}
		// Ensure the email isn't taken by another account.
		exists, err := u.authRepo.EmailExistsForOtherUser(req.Email, userID)
		if err != nil {
			return nil, errors.New("failed to validate email")
		}
		if exists {
			return nil, errors.New("email already in use")
		}
		fields["email"] = strings.TrimSpace(req.Email)
	}
	// Only fields the client actually sent — an omitted one keeps its stored value.
	if req.StoreName != nil {
		fields["store_name"] = *req.StoreName
	}
	if req.Address != nil {
		fields["address"] = *req.Address
	}
	if req.TaxID != nil {
		fields["tax_id"] = *req.TaxID
	}
	if req.BankID != nil {
		fields["bank_id"] = normalizeBankID(req.BankID)
	}
	if req.BankAccountNo != nil {
		fields["bank_account_no"] = *req.BankAccountNo
	}
	if req.BankAccountName != nil {
		fields["bank_account_name"] = *req.BankAccountName
	}
	if err := u.authRepo.UpdateProfile(userID, fields); err != nil {
		return nil, errors.New("failed to update profile")
	}

	return u.GetMe(userID)
}

func (u *authUsecase) ChangePassword(userID uint, req *ChangePasswordRequest) error {
	if req.OldPassword == "" {
		return errors.New("current password is required")
	}
	if len(req.NewPassword) < 6 {
		return errors.New("new password must be at least 6 characters")
	}

	user, err := u.authRepo.FindByIDWithRole(userID)
	if err != nil {
		return errors.New("user not found")
	}

	if !jwtPkg.CheckPassword(user.Password, req.OldPassword) {
		return errors.New("current password is incorrect")
	}

	hashed, err := jwtPkg.HashPassword(req.NewPassword)
	if err != nil {
		return errors.New("failed to hash password")
	}

	if err := u.authRepo.UpdateProfile(userID, map[string]interface{}{"password": hashed}); err != nil {
		return errors.New("failed to update password")
	}
	return nil
}

func (u *authUsecase) UpdateAvatar(userID uint, path string) (*MeResponse, error) {
	if err := u.authRepo.UpdateProfile(userID, map[string]interface{}{"avatar": path}); err != nil {
		return nil, errors.New("failed to update avatar")
	}
	return u.GetMe(userID)
}

func (u *authUsecase) RefreshToken(userID uint) (*TokenResponse, error) {
	user, err := u.authRepo.FindByIDWithRole(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	roleName := ""
	var roleID uint
	if user.Role != nil {
		roleName = user.Role.Name
		roleID = user.Role.ID
	}

	claims := &jwtPkg.Claims{
		UserID:   user.ID,
		StoreID:  user.StoreID,
		BranchID: user.BranchID,
		RoleID:   roleID,
		RoleName: roleName,
	}

	token, err := jwtPkg.GenerateToken(u.jwtSecret, u.jwtExpiry, claims)
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	return &TokenResponse{Token: token}, nil
}

const (
	pdpaConsentType      = "pdpa"
	marketingConsentType = "marketing"
)

// currentPDPA reads the shop's published consent text and its version.
// ok is false when the feature is effectively switched off: an empty text (the
// documented kill switch) or a version that is missing or unparseable. Failing
// closed here matters — a typo in the version field must not lock every customer
// out behind a modal they cannot satisfy.
func (u *authUsecase) currentPDPA() (text string, version int, ok bool) {
	text = strings.TrimSpace(u.authRepo.GetConfigValue("pdpa_consent_text"))
	version, err := strconv.Atoi(strings.TrimSpace(u.authRepo.GetConfigValue("pdpa_consent_version")))
	if text == "" || err != nil || version <= 0 {
		return "", 0, false
	}
	return text, version, true
}

// pdpaStatusFor returns what the client needs to show the consent modal, or nil
// when there is nothing to ask.
//
// Only customers are asked. Staff accounts are the shop's own people, whose data
// handling is covered by employment rather than by a consent they tick — and
// gating the back office behind this modal would stop the shop from working the
// moment someone bumped the version.
func (u *authUsecase) pdpaStatusFor(user *entity.User) *PDPAStatus {
	if user == nil || user.Role == nil || user.Role.Name != "customer" {
		return nil
	}
	text, version, ok := u.currentPDPA()
	if !ok {
		return nil
	}
	if u.authRepo.HasConsent(user.ID, pdpaConsentType, version) {
		return nil
	}
	return &PDPAStatus{Required: true, Version: version, Text: text}
}

// AcceptPDPA records the acknowledgement, and nothing else. The version and the
// text come from the server, never from the request: the client may only say
// "I read it", not what it read, or a stale tab could file evidence against the
// wrong text.
//
// The marketing consent deliberately does NOT ride along here. Asking for it in
// the one window a customer cannot dismiss turns a simple "read this" into a
// decision, and a blocking screen is the worst place to make somebody weigh one.
// It lives on the profile page instead, where saying yes is a choice rather than
// an obstacle.
func (u *authUsecase) AcceptPDPA(userID uint, ip, userAgent string) error {
	text, version, ok := u.currentPDPA()
	if !ok {
		return errors.New("ยังไม่ได้ตั้งค่าข้อความประกาศความเป็นส่วนตัว")
	}
	return u.authRepo.CreateAcknowledgement(&entity.UserConsent{
		UserID:       userID,
		ConsentType:  pdpaConsentType,
		Version:      version,
		Granted:      true,
		TextSnapshot: text,
		IP:           ip,
		UserAgent:    userAgent,
		AcceptedAt:   time.Now(),
	})
}

// marketingText is the shop's published opt-in wording; empty switches the whole
// optional consent off, the same way an empty notice switches the gate off.
func (u *authUsecase) marketingText() string {
	return strings.TrimSpace(u.authRepo.GetConfigValue("pdpa_marketing_text"))
}

func (u *authUsecase) marketingGranted(userID uint) bool {
	row, ok := u.authRepo.LatestConsent(userID, marketingConsentType)
	return ok && row.Granted
}

func (u *authUsecase) MarketingConsent(userID uint) MarketingConsentStatus {
	text := u.marketingText()
	if text == "" {
		return MarketingConsentStatus{}
	}
	return MarketingConsentStatus{
		Available: true,
		Granted:   u.marketingGranted(userID),
		Text:      text,
	}
}

// SetMarketingConsent appends the new answer instead of editing the old one, so
// the log reads "given on X, withdrawn on Y" rather than only ever showing where
// the customer landed. Re-stating the answer they already hold is a no-op — a
// toggle flipped twice must not read as two separate decisions.
func (u *authUsecase) SetMarketingConsent(userID uint, granted bool, ip, userAgent string) error {
	text := u.marketingText()
	if text == "" {
		return errors.New("ร้านยังไม่ได้เปิดรับความยินยอมด้านการตลาด")
	}
	if row, ok := u.authRepo.LatestConsent(userID, marketingConsentType); ok && row.Granted == granted {
		return nil
	}
	// Only a grant snapshots the wording: a withdrawal is an answer to whatever
	// was consented to before, not consent to the text standing today.
	snapshot := ""
	if granted {
		snapshot = text
	}
	_, version, _ := u.currentPDPA()
	return u.authRepo.AppendConsent(&entity.UserConsent{
		UserID:       userID,
		ConsentType:  marketingConsentType,
		Version:      version,
		Granted:      granted,
		TextSnapshot: snapshot,
		IP:           ip,
		UserAgent:    userAgent,
		AcceptedAt:   time.Now(),
	})
}
