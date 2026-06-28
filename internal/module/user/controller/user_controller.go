package controller

import (
	"strconv"

	"jk-api/internal/middleware"
	"jk-api/internal/module/user/repository"
	"jk-api/internal/module/user/usecase"
	"jk-api/pkg/response"
	"jk-api/pkg/upload"

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

	role := middleware.GetRoleName(c)
	var f repository.UserFilter

	switch role {
	case "master":
		// master sees all — honour every query param
		if v := c.Query("store_id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				uid := uint(id)
				f.StoreID = &uid
			}
		}
		if v := c.Query("branch_id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				uid := uint(id)
				f.BranchID = &uid
			}
		}
	case "owner":
		// owner is locked to their own store
		storeID := middleware.GetStoreID(c)
		f.StoreID = storeID
		// may optionally filter by branch inside their store
		if v := c.Query("branch_id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				uid := uint(id)
				f.BranchID = &uid
			}
		}
	default:
		// employee — locked to store + branch
		f.StoreID = middleware.GetStoreID(c)
		f.BranchID = middleware.GetBranchID(c)
	}

	// common filters
	if v := c.Query("role_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 32); err == nil {
			uid := uint(id)
			f.RoleID = &uid
		}
	}
	if v := c.Query("search"); v != "" {
		f.Search = v
	}
	if v := c.Query("is_active"); v != "" {
		active := v == "true"
		f.IsActive = &active
	}

	users, total, err := ctrl.userUsecase.GetAllUsers(page, limit, f)
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

func (ctrl *UserController) UploadAvatar(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	path, err := upload.SaveFile(c, "avatar", "avatars")
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	user, err := ctrl.userUsecase.UpdateAvatar(uint(id), path)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Avatar uploaded", user)
}

func (ctrl *UserController) CheckEmail(c *fiber.Ctx) error {
	email := c.Query("email")
	if email == "" {
		return response.BadRequest(c, "email query required")
	}
	exists := ctrl.userUsecase.CheckEmailExists(email)
	return response.Success(c, "ok", fiber.Map{"exists": exists})
}

func (ctrl *UserController) CheckStoreOwner(c *fiber.Ctx) error {
	storeIDStr := c.Query("store_id")
	if storeIDStr == "" {
		return response.BadRequest(c, "store_id query required")
	}
	id, err := strconv.ParseUint(storeIDStr, 10, 32)
	if err != nil {
		return response.BadRequest(c, "invalid store_id")
	}
	hasOwner := ctrl.userUsecase.CheckStoreHasOwner(uint(id))
	return response.Success(c, "ok", fiber.Map{"has_owner": hasOwner})
}
