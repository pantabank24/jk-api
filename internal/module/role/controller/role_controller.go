package controller

import (
	"strconv"

	"jk-api/internal/module/role/usecase"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type RoleController struct {
	roleUsecase usecase.RoleUsecase
}

func NewRoleController(roleUsecase usecase.RoleUsecase) *RoleController {
	return &RoleController{roleUsecase: roleUsecase}
}

func (ctrl *RoleController) GetAllRoles(c *fiber.Ctx) error {
	roles, err := ctrl.roleUsecase.GetAllRoles()
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Roles retrieved", roles)
}

func (ctrl *RoleController) GetRoleByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid role ID")
	}

	role, err := ctrl.roleUsecase.GetRoleByID(uint(id))
	if err != nil {
		return response.NotFound(c, "Role not found")
	}
	return response.Success(c, "Role retrieved", role)
}

func (ctrl *RoleController) CreateRole(c *fiber.Ctx) error {
	var req usecase.CreateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if req.Name == "" || req.DisplayName == "" {
		return response.BadRequest(c, "Name and display name are required")
	}

	role, err := ctrl.roleUsecase.CreateRole(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Role created", role)
}

func (ctrl *RoleController) UpdateRole(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid role ID")
	}

	var req usecase.UpdateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	role, err := ctrl.roleUsecase.UpdateRole(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Role updated", role)
}

func (ctrl *RoleController) DeleteRole(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid role ID")
	}

	if err := ctrl.roleUsecase.DeleteRole(uint(id)); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Role deleted", nil)
}

func (ctrl *RoleController) GetAllPermissions(c *fiber.Ctx) error {
	permissions, err := ctrl.roleUsecase.GetAllPermissions()
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Permissions retrieved", permissions)
}

func (ctrl *RoleController) SetRolePermissions(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid role ID")
	}

	var req usecase.SetPermissionsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := ctrl.roleUsecase.SetRolePermissions(uint(id), &req); err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Permissions updated", nil)
}
