package controller

import (
	"jk-api/internal/entity"
	"jk-api/internal/middleware"
	"jk-api/internal/module/auth/usecase"
	logRepo "jk-api/internal/module/log/repository"
	"jk-api/pkg/response"
	"jk-api/pkg/upload"
	"jk-api/pkg/useragent"

	"github.com/gofiber/fiber/v2"
)

type AuthController struct {
	authUsecase usecase.AuthUsecase
	logRepo     logRepo.LogRepository
}

func NewAuthController(authUsecase usecase.AuthUsecase, logRepo logRepo.LogRepository) *AuthController {
	return &AuthController{authUsecase: authUsecase, logRepo: logRepo}
}

func (ctrl *AuthController) Login(c *fiber.Ctx) error {
	var req usecase.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if req.Email == "" || req.Password == "" {
		return response.BadRequest(c, "Email and password are required")
	}

	result, err := ctrl.authUsecase.Login(&req)

	// Record login log asynchronously
	loginLog := &entity.LoginLog{
		Email:     req.Email,
		IP:        c.IP(),
		UserAgent: c.Get("User-Agent"),
		Device:    useragent.ParseDevice(c.Get("User-Agent")),
		Success:   err == nil,
	}
	if err != nil {
		loginLog.FailReason = err.Error()
	} else if result != nil {
		if u, ok := result.User.(*entity.User); ok && u != nil {
			uid := u.ID
			loginLog.UserID = &uid
		}
	}
	go func() { _ = ctrl.logRepo.CreateLoginLog(loginLog) }()

	if err != nil {
		return response.Unauthorized(c, err.Error())
	}
	return response.Success(c, "Login successful", result)
}

func (ctrl *AuthController) GetMe(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	result, err := ctrl.authUsecase.GetMe(userID)
	if err != nil {
		return response.NotFound(c, err.Error())
	}

	return response.Success(c, "User info retrieved", result)
}

// AcceptPDPAConsent records that the customer read the privacy notice. The body
// is ignored on purpose — the version and text being acknowledged are the
// server's current ones, and the evidence (IP, user agent) is taken from the
// request itself rather than from anything the client could choose to send.
func (ctrl *AuthController) AcceptPDPAConsent(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if err := ctrl.authUsecase.AcceptPDPA(userID, c.IP(), c.Get("User-Agent")); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "บันทึกแล้ว", nil)
}

// GetMarketingConsent drives the toggle on the customer's own profile.
func (ctrl *AuthController) GetMarketingConsent(c *fiber.Ctx) error {
	status := ctrl.authUsecase.MarketingConsent(middleware.GetUserID(c))
	return response.Success(c, "Marketing consent", status)
}

// UpdateMarketingConsent gives or withdraws it. PDPA expects withdrawing to be
// as easy as giving, which is why this is a plain toggle on a page the customer
// already visits rather than a request they have to make to the shop.
func (ctrl *AuthController) UpdateMarketingConsent(c *fiber.Ctx) error {
	var req struct {
		Granted bool `json:"granted"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	userID := middleware.GetUserID(c)
	if err := ctrl.authUsecase.SetMarketingConsent(userID, req.Granted, c.IP(), c.Get("User-Agent")); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "บันทึกแล้ว", ctrl.authUsecase.MarketingConsent(userID))
}

func (ctrl *AuthController) UpdateProfile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req usecase.UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// A customer's email is their issued login — read-only to them (the form
	// disables it); staff may still change their own.
	allowEmailChange := middleware.GetRoleName(c) != "customer"

	result, err := ctrl.authUsecase.UpdateProfile(userID, &req, allowEmailChange)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.Success(c, "Profile updated", result)
}

func (ctrl *AuthController) ChangePassword(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req usecase.ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := ctrl.authUsecase.ChangePassword(userID, &req); err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.Success(c, "Password changed", nil)
}

func (ctrl *AuthController) UploadAvatar(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	path, err := upload.SaveFile(c, "avatar", "avatars")
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	result, err := ctrl.authUsecase.UpdateAvatar(userID, path)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.Success(c, "Avatar uploaded", result)
}

func (ctrl *AuthController) RefreshToken(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	result, err := ctrl.authUsecase.RefreshToken(userID)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.Success(c, "Token refreshed", result)
}
