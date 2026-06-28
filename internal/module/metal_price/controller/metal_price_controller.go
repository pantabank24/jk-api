package controller

import (
	"math"
	"strconv"
	"time"

	"jk-api/internal/entity"
	"jk-api/internal/module/metal_price/repository"
	"jk-api/pkg/response"
	"jk-api/pkg/silverprice"

	"github.com/gofiber/fiber/v2"
)

const symbolSilver = "XAG"

type MetalPriceController struct {
	repo repository.MetalPriceRepository
}

func NewMetalPriceController(repo repository.MetalPriceRepository) *MetalPriceController {
	return &MetalPriceController{repo: repo}
}

// GetLatest returns the latest price for ?symbol= (default XAG / silver).
func (ctrl *MetalPriceController) GetLatest(c *fiber.Ctx) error {
	symbol := c.Query("symbol", symbolSilver)
	mp, err := ctrl.repo.GetLatest(symbol)
	if err != nil {
		return response.Success(c, "ยังไม่มีข้อมูลราคา", nil)
	}
	return response.Success(c, "Metal price retrieved", mp)
}

func (ctrl *MetalPriceController) GetHistory(c *fiber.Ctx) error {
	symbol := c.Query("symbol", symbolSilver)
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	prices, err := ctrl.repo.GetHistory(symbol, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Metal price history retrieved", prices)
}

type ManualPriceRequest struct {
	Symbol     string     `json:"symbol"`
	Buy        float64    `json:"buy"`
	Sell       float64    `json:"sell"`
	Spot       float64    `json:"spot"`
	ValidFrom  *time.Time `json:"valid_from"`
	ValidUntil *time.Time `json:"valid_until"`
}

// SetManual stores a manually-entered metal price (fallback when the auto feed
// is unavailable). Recorded with source=manual.
func (ctrl *MetalPriceController) SetManual(c *fiber.Ctx) error {
	var req ManualPriceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}
	if req.Symbol == "" {
		req.Symbol = symbolSilver
	}
	if req.ValidFrom == nil || req.ValidUntil == nil {
		return response.BadRequest(c, "กรุณาระบุช่วงเวลาที่ใช้ราคานี้ (ตั้งแต่–ถึง)")
	}
	if req.ValidUntil.Before(*req.ValidFrom) {
		return response.BadRequest(c, "เวลาสิ้นสุดต้องอยู่หลังเวลาเริ่ม")
	}
	now := time.Now()
	mp := &entity.MetalPrice{
		Symbol:     req.Symbol,
		Buy:        math.Floor(req.Buy),
		Sell:       math.Floor(req.Sell),
		Spot:       math.Floor(req.Spot),
		Source:     "manual",
		PriceDate:  now.Format("2006-01-02"),
		PriceTime:  now.Format("15:04"),
		ValidFrom:  req.ValidFrom,
		ValidUntil: req.ValidUntil,
		CreatedAt:  now,
	}
	if err := ctrl.repo.Create(mp); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Created(c, "บันทึกราคาแล้ว", mp)
}

// FetchAndSave scrapes the latest silver price and stores it.
func (ctrl *MetalPriceController) FetchAndSave(c *fiber.Ctx) error {
	data, err := silverprice.Fetch()
	if err != nil {
		return response.BadRequest(c, "ดึงราคาเงินไม่สำเร็จ: "+err.Error())
	}
	mp := &entity.MetalPrice{
		Symbol:    symbolSilver,
		Buy:       data.Buy,
		Sell:      data.Sell,
		Spot:      data.Spot,
		Exchange:  data.Exchange,
		Previous:  data.Previous,
		Round:     data.Round,
		PriceDate: data.Date,
		Source:    "auto",
		CreatedAt: time.Now(),
	}
	if err := ctrl.repo.Create(mp); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Created(c, "Silver price fetched and saved", mp)
}
