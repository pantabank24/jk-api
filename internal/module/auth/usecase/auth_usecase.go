package usecase

import (
	"errors"
	"time"

	"jk-api/internal/module/auth/repository"
	jwtPkg "jk-api/pkg/jwt"
)

type AuthUsecase interface {
	Login(req *LoginRequest) (*LoginResponse, error)
	GetMe(userID uint) (*MeResponse, error)
	RefreshToken(userID uint) (*TokenResponse, error)
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	Token       string      `json:"token"`
	User        interface{} `json:"user"`
	Permissions []string    `json:"permissions"`
}

type MeResponse struct {
	User        interface{} `json:"user"`
	Permissions []string    `json:"permissions"`
}

type TokenResponse struct {
	Token string `json:"token"`
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

	return &LoginResponse{
		Token:       token,
		User:        user,
		Permissions: permissions,
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

	return &MeResponse{
		User:        user,
		Permissions: permissions,
	}, nil
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
