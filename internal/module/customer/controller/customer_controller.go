package controller

import (
	"strconv"

	"jk-api/internal/middleware"
	"jk-api/internal/module/customer/usecase"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type CustomerController struct {
	customerUsecase usecase.CustomerUsecase
}

func NewCustomerController(customerUsecase usecase.CustomerUsecase) *CustomerController {
	return &CustomerController{customerUsecase: customerUsecase}
}

func (ctrl *CustomerController) CreateCustomer(c *fiber.Ctx) error {
	var req usecase.CreateCustomerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	customer, err := ctrl.customerUsecase.CreateCustomer(&req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Customer created", customer)
}

func (ctrl *CustomerController) GetAllCustomers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := c.Query("search", "")

	var storeID, branchID *uint
	roleName := middleware.GetRoleName(c)
	switch roleName {
	case "master":
		if v := c.Query("store_id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				uid := uint(id)
				storeID = &uid
			}
		}
		if v := c.Query("branch_id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				uid := uint(id)
				branchID = &uid
			}
		}
	case "owner":
		storeID = middleware.GetStoreID(c)
		if v := c.Query("branch_id"); v != "" {
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				uid := uint(id)
				branchID = &uid
			}
		}
	default: // employee
		storeID = middleware.GetStoreID(c)
		branchID = middleware.GetBranchID(c)
	}

	customers, total, err := ctrl.customerUsecase.GetAllCustomers(page, limit, storeID, branchID, search)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Customers retrieved", customers, page, limit, total)
}

func (ctrl *CustomerController) GetCustomerByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}
	customer, err := ctrl.customerUsecase.GetCustomerByID(uint(id))
	if err != nil {
		return response.NotFound(c, "Customer not found")
	}
	return response.Success(c, "Customer retrieved", customer)
}

func (ctrl *CustomerController) UpdateCustomer(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}
	var req usecase.UpdateCustomerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	customer, err := ctrl.customerUsecase.UpdateCustomer(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Customer updated", customer)
}

func (ctrl *CustomerController) DeleteCustomer(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}
	if err := ctrl.customerUsecase.DeleteCustomer(uint(id)); err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.Success(c, "Customer deleted", nil)
}
