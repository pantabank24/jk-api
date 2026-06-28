package controller

import (
	"math"
	"strconv"
	"time"

	"jk-api/internal/entity"
	"jk-api/internal/module/gold_price/repository"
	"jk-api/pkg/goldprice"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type GoldPriceController struct {
	repo repository.GoldPriceRepository
}

func NewGoldPriceController(repo repository.GoldPriceRepository) *GoldPriceController {
	return &GoldPriceController{repo: repo}
}

func (ctrl *GoldPriceController) GetLatest(c *fiber.Ctx) error {
	gp, err := ctrl.repo.GetLatest()
	if err != nil {
		return response.Success(c, "ยังไม่มีข้อมูลราคาทอง", nil)
	}
	return response.Success(c, "Gold price retrieved", gp)
}

func (ctrl *GoldPriceController) GetHistory(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	prices, err := ctrl.repo.GetHistory(limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Gold price history retrieved", prices)
}

// FetchAndSave scrapes the latest gold price and stores it
func (ctrl *GoldPriceController) FetchAndSave(c *fiber.Ctx) error {
	data, err := goldprice.Fetch()
	if err != nil {
		return response.BadRequest(c, "ดึงราคาทองไม่สำเร็จ: "+err.Error())
	}

	gp := &entity.GoldPrice{
		BarBuy:          data.BarBuy,
		BarSell:         data.BarSell,
		OrnamentBuy:     data.OrnamentBuy,
		OrnamentSell:    data.OrnamentSell,
		ChangeToday:     data.ChangeToday,
		ChangeYesterday: data.ChangeYesterday,
		GoldDate:        data.GoldDate,
		GoldTime:        data.GoldTime,
		GoldRound:       data.GoldRound,
		CreatedAt:       time.Now(),
	}
	if err := ctrl.repo.Create(gp); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Created(c, "Gold price fetched and saved", gp)
}

type ManualGoldRequest struct {
	BarBuy       float64    `json:"bar_buy"`
	BarSell      float64    `json:"bar_sell"`
	OrnamentBuy  float64    `json:"ornament_buy"`
	OrnamentSell float64    `json:"ornament_sell"`
	ValidFrom    *time.Time `json:"valid_from"`
	ValidUntil   *time.Time `json:"valid_until"`
}

// SetManual stores a manually-entered gold price valid for a time window. While
// the window is active it overrides the auto-fetched price; afterwards the
// system falls back to auto.
func (ctrl *GoldPriceController) SetManual(c *fiber.Ctx) error {
	var req ManualGoldRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}
	if req.ValidFrom == nil || req.ValidUntil == nil {
		return response.BadRequest(c, "กรุณาระบุช่วงเวลาที่ใช้ราคานี้ (ตั้งแต่–ถึง)")
	}
	if req.ValidUntil.Before(*req.ValidFrom) {
		return response.BadRequest(c, "เวลาสิ้นสุดต้องอยู่หลังเวลาเริ่ม")
	}
	now := time.Now()
	gp := &entity.GoldPrice{
		BarBuy:       math.Round(req.BarBuy),
		BarSell:      math.Round(req.BarSell),
		OrnamentBuy:  math.Round(req.OrnamentBuy),
		OrnamentSell: math.Round(req.OrnamentSell),
		GoldDate:     now.Format("2006-01-02"),
		GoldTime:     now.Format("15:04"),
		GoldRound:    "manual",
		Source:       "manual",
		ValidFrom:    req.ValidFrom,
		ValidUntil:   req.ValidUntil,
		CreatedAt:    now,
	}
	if err := ctrl.repo.Create(gp); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Created(c, "บันทึกราคาทองแล้ว", gp)
}
