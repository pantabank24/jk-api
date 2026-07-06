package usecase

import (
	"errors"
	"strings"
	"time"

	"jk-api/internal/module/auth/repository"
	jwtPkg "jk-api/pkg/jwt"
)

type AuthUsecase interface {
	Login(req *LoginRequest) (*LoginResponse, error)
	GetMe(userID uint) (*MeResponse, error)
	RefreshToken(userID uint) (*TokenResponse, error)
	UpdateProfile(userID uint, req *UpdateProfileRequest) (*MeResponse, error)
	ChangePassword(userID uint, req *ChangePasswordRequest) error
	UpdateAvatar(userID uint, path string) (*MeResponse, error)
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
}

type MeResponse struct {
	User        interface{} `json:"user"`
	Permissions []string    `json:"permissions"`
	Credits     float64     `json:"credits"`
}

type TokenResponse struct {
	Token string `json:"token"`
}

type UpdateProfileRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
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

	return &LoginResponse{
		Token:       token,
		User:        user,
		Permissions: permissions,
		Credits:     credits,
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
	}, nil
}

func (u *authUsecase) UpdateProfile(userID uint, req *UpdateProfileRequest) (*MeResponse, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("name is required")
	}
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

	fields := map[string]interface{}{
		"name":  strings.TrimSpace(req.Name),
		"email": strings.TrimSpace(req.Email),
		"phone": req.Phone,
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
