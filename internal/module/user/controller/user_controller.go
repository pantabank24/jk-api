package controller

import (
	"strconv"

	"jk-api/internal/module/user/usecase"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type UserController struct {
	userUsecase usecase.UserUsecase
}

func NewUserController(userUsecase usecase.UserUsecase) *UserController {
	return &UserController{userUsecase: userUsecase}
}

// CreateUser godoc
// @Summary      Create a new user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body  usecase.CreateUserRequest  true  "Create user request"
// @Success      201   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Router       /api/v1/users [post]
func (ctrl *UserController) CreateUser(c *fiber.Ctx) error {
	var req usecase.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	user, err := ctrl.userUsecase.CreateUser(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.Created(c, "User created successfully", user)
}

// GetAllUsers godoc
// @Summary      Get all users
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        page   query  int  false  "Page number"  default(1)
// @Param        limit  query  int  false  "Items per page"  default(10)
// @Success      200    {object}  response.PaginatedResponse
// @Router       /api/v1/users [get]
func (ctrl *UserController) GetAllUsers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	users, total, err := ctrl.userUsecase.GetAllUsers(page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.Paginated(c, "Users retrieved successfully", users, page, limit, total)
}

// GetUserByID godoc
// @Summary      Get user by ID
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id  path  int  true  "User ID"
// @Success      200 {object}  response.Response
// @Failure      404 {object}  response.Response
// @Router       /api/v1/users/{id} [get]
func (ctrl *UserController) GetUserByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	user, err := ctrl.userUsecase.GetUserByID(uint(id))
	if err != nil {
		return response.NotFound(c, "User not found")
	}

	return response.Success(c, "User retrieved successfully", user)
}

// UpdateUser godoc
// @Summary      Update user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path  int  true  "User ID"
// @Param        body  body  usecase.UpdateUserRequest  true  "Update user request"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Router       /api/v1/users/{id} [put]
func (ctrl *UserController) UpdateUser(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	var req usecase.UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	user, err := ctrl.userUsecase.UpdateUser(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.Success(c, "User updated successfully", user)
}

// DeleteUser godoc
// @Summary      Delete user
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id  path  int  true  "User ID"
// @Success      200 {object}  response.Response
// @Failure      404 {object}  response.Response
// @Router       /api/v1/users/{id} [delete]
func (ctrl *UserController) DeleteUser(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	if err := ctrl.userUsecase.DeleteUser(uint(id)); err != nil {
		return response.NotFound(c, err.Error())
	}

	return response.Success(c, "User deleted successfully", nil)
}
