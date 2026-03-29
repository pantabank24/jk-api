package controller

import (
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
