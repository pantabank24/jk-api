package controller

import (
	"jk-api/internal/entity"
	"jk-api/internal/middleware"
	"jk-api/internal/module/auth/usecase"
	logRepo "jk-api/internal/module/log/repository"
	"jk-api/pkg/response"
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

func (ctrl *AuthController) RefreshToken(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	result, err := ctrl.authUsecase.RefreshToken(userID)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.Success(c, "Token refreshed", result)
}
