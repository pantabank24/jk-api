package controller

import (
	"strconv"

	"jk-api/internal/module/branch/usecase"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type BranchController struct {
	branchUsecase usecase.BranchUsecase
}

func NewBranchController(branchUsecase usecase.BranchUsecase) *BranchController {
	return &BranchController{branchUsecase: branchUsecase}
}

func (ctrl *BranchController) CreateBranch(c *fiber.Ctx) error {
	storeID, err := strconv.ParseUint(c.Params("storeId"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid store ID")
	}

	var req usecase.CreateBranchRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if req.Name == "" {
		return response.BadRequest(c, "Branch name is required")
	}

	branch, err := ctrl.branchUsecase.CreateBranch(uint(storeID), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Branch created", branch)
}

func (ctrl *BranchController) GetAllBranches(c *fiber.Ctx) error {
	storeID, err := strconv.ParseUint(c.Params("storeId"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid store ID")
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	branches, total, err := ctrl.branchUsecase.GetAllBranches(uint(storeID), page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Branches retrieved", branches, page, limit, total)
}

func (ctrl *BranchController) GetBranchByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid branch ID")
	}

	branch, err := ctrl.branchUsecase.GetBranchByID(uint(id))
	if err != nil {
		return response.NotFound(c, "Branch not found")
	}
	return response.Success(c, "Branch retrieved", branch)
}

func (ctrl *BranchController) UpdateBranch(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid branch ID")
	}

	var req usecase.UpdateBranchRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	branch, err := ctrl.branchUsecase.UpdateBranch(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Branch updated", branch)
}

func (ctrl *BranchController) DeleteBranch(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid branch ID")
	}

	if err := ctrl.branchUsecase.DeleteBranch(uint(id)); err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.Success(c, "Branch deleted", nil)
}
