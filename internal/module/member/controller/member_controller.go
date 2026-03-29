package controller

import (
	"strconv"

	"jk-api/internal/middleware"
	memberUC "jk-api/internal/module/member/usecase"
	userUC "jk-api/internal/module/user/usecase"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type MemberController struct {
	memberUsecase memberUC.MemberUsecase
	userUsecase   userUC.UserUsecase
}

func NewMemberController(memberUsecase memberUC.MemberUsecase, userUsecase userUC.UserUsecase) *MemberController {
	return &MemberController{memberUsecase: memberUsecase, userUsecase: userUsecase}
}

// createMemberBody is the combined request for member + optional user account
type createMemberBody struct {
	// Member profile fields
	Fname    string  `json:"fname"`
	Lname    string  `json:"lname"`
	Phone    string  `json:"phone"`
	Credits  float64 `json:"credits"`
	StoreID  uint    `json:"store_id"`
	BranchID uint    `json:"branch_id"`
	// User account fields (optional — if provided, creates a linked user account)
	Email    string `json:"email"`
	Password string `json:"password"`
	RoleID   *uint  `json:"role_id"`
}

func (ctrl *MemberController) CreateMember(c *fiber.Ctx) error {
	var body createMemberBody
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	if body.Fname == "" || body.Lname == "" {
		return response.BadRequest(c, "กรุณาระบุชื่อและนามสกุล")
	}

	// Auto-fill store/branch from JWT if not provided
	if body.StoreID == 0 {
		if storeID := middleware.GetStoreID(c); storeID != nil {
			body.StoreID = *storeID
		}
	}
	if body.BranchID == 0 {
		if branchID := middleware.GetBranchID(c); branchID != nil {
			body.BranchID = *branchID
		}
	}

	memberReq := &memberUC.CreateMemberRequest{
		StoreID:  body.StoreID,
		BranchID: body.BranchID,
		Fname:    body.Fname,
		Lname:    body.Lname,
		Phone:    body.Phone,
		Credits:  body.Credits,
	}

	// If email + password provided → create user account first, then link it
	if body.Email != "" && body.Password != "" {
		userReq := &userUC.CreateUserRequest{
			Name:     body.Fname + " " + body.Lname,
			Email:    body.Email,
			Password: body.Password,
			Phone:    body.Phone,
			RoleID:   body.RoleID,
		}
		if body.StoreID != 0 {
			storeID := body.StoreID
			userReq.StoreID = &storeID
		}
		if body.BranchID != 0 {
			branchID := body.BranchID
			userReq.BranchID = &branchID
		}

		user, err := ctrl.userUsecase.CreateUser(userReq)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		memberReq.UserID = &user.ID
	}

	member, err := ctrl.memberUsecase.CreateMember(memberReq)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Created(c, "Member created", member)
}

func (ctrl *MemberController) GetAllMembers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := c.Query("search", "")

	var storeID, branchID *uint
	if !middleware.IsMaster(c) {
		storeID = middleware.GetStoreID(c)
		roleName := middleware.GetRoleName(c)
		if roleName != "owner" {
			branchID = middleware.GetBranchID(c)
		}
	} else {
		if sid := c.Query("store_id"); sid != "" {
			id, _ := strconv.ParseUint(sid, 10, 32)
			uid := uint(id)
			storeID = &uid
		}
		if bid := c.Query("branch_id"); bid != "" {
			id, _ := strconv.ParseUint(bid, 10, 32)
			uid := uint(id)
			branchID = &uid
		}
	}

	members, total, err := ctrl.memberUsecase.GetAllMembers(storeID, branchID, page, limit, search)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Members retrieved", members, page, limit, total)
}

func (ctrl *MemberController) GetMemberByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid member ID")
	}

	member, err := ctrl.memberUsecase.GetMemberByID(uint(id))
	if err != nil {
		return response.NotFound(c, "Member not found")
	}
	return response.Success(c, "Member retrieved", member)
}

func (ctrl *MemberController) UpdateMember(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid member ID")
	}

	var req memberUC.UpdateMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	member, err := ctrl.memberUsecase.UpdateMember(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Member updated", member)
}

func (ctrl *MemberController) DeleteMember(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid member ID")
	}

	if err := ctrl.memberUsecase.DeleteMember(uint(id)); err != nil {
		return response.NotFound(c, err.Error())
	}
	return response.Success(c, "Member deleted", nil)
}

func (ctrl *MemberController) AddCredit(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid member ID")
	}

	var req memberUC.CreditRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	userID := middleware.GetUserID(c)
	req.CreatedBy = &userID
	if storeID := middleware.GetStoreID(c); storeID != nil {
		req.StoreID = *storeID
	}
	if branchID := middleware.GetBranchID(c); branchID != nil {
		req.BranchID = *branchID
	}

	member, err := ctrl.memberUsecase.AddCredit(uint(id), &req)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	return response.Success(c, "Credit updated", member)
}

func (ctrl *MemberController) GetCreditTransactions(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid member ID")
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	txs, total, err := ctrl.memberUsecase.GetCreditTransactions(uint(id), page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Transactions retrieved", txs, page, limit, total)
}

func (ctrl *MemberController) GetAllCreditTransactions(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	source := c.Query("source", "")
	search := c.Query("search", "")

	var storeID, branchID, memberID *uint

	if !middleware.IsMaster(c) {
		storeID = middleware.GetStoreID(c)
		roleName := middleware.GetRoleName(c)
		if roleName != "owner" {
			branchID = middleware.GetBranchID(c)
		}
	} else {
		if sid := c.Query("store_id"); sid != "" {
			id, _ := strconv.ParseUint(sid, 10, 32)
			uid := uint(id)
			storeID = &uid
		}
		if bid := c.Query("branch_id"); bid != "" {
			id, _ := strconv.ParseUint(bid, 10, 32)
			uid := uint(id)
			branchID = &uid
		}
	}

	if mid := c.Query("member_id"); mid != "" {
		id, _ := strconv.ParseUint(mid, 10, 32)
		uid := uint(id)
		memberID = &uid
	}

	txs, total, err := ctrl.memberUsecase.GetAllCreditTransactions(storeID, branchID, memberID, source, page, limit, search)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Paginated(c, "Transactions retrieved", txs, page, limit, total)
}
