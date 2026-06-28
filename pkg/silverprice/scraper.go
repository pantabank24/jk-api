package silverprice

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// ScrapedData is the latest silver bar price expressed the same way the legacy
// jk-goldtrader feed was: Buy is THB per kilogram of pure silver, which the
// quotation formula divides by 1000 to get THB/gram.
type ScrapedData struct {
	Buy      float64
	Sell     float64
	Spot     float64 // silver spot, USD per troy ounce
	Exchange float64 // USD -> THB rate used for the conversion
	Previous float64
	Round    string
	Date     string
}

// jk-goldtrader's own silver source (besserver.dyndns.org) is offline, so we
// reconstruct the same value from the silver spot price (XAG, USD/oz — the same
// quote jk-goldtrader charts via TradingView) converted to THB/kg.
const (
	xagURL  = "https://api.gold-api.com/price/XAG"    // { price: USD/oz }
	fxURL   = "https://open.er-api.com/v6/latest/USD" // { rates: { THB } }
	gramsOz = 31.1034768                              // grams per troy ounce

	// Dealer bid/ask spread applied to the spot mid (THB/kg) to reproduce the
	// shop's quoted buy/sell — the spread that the legacy besserver feed baked
	// in. Calibrated to buy 63,200 / sell 64,700 at a mid of 63,631. Adjust these
	// if the dealer's spread changes.
	buyFactor  = 0.99323 // buy  = mid * buyFactor  (slightly below spot)
	sellFactor = 1.01680 // sell = mid * sellFactor (slightly above spot)
)

func Fetch() (*ScrapedData, error) {
	spot, updatedAt, err := fetchSpotUSD()
	if err != nil {
		return nil, fmt.Errorf("ดึงราคา spot เงินไม่สำเร็จ: %w", err)
	}
	thb, err := fetchUSDTHB()
	if err != nil {
		return nil, fmt.Errorf("ดึงอัตราแลกเปลี่ยนไม่สำเร็จ: %w", err)
	}

	// Spot mid: THB per kg of pure silver = (USD/oz) * (THB/USD) * (1000 g/kg) / (g/oz).
	// Then apply the dealer bid/ask spread to get buy/sell.
	midPerKg := spot * thb * 1000.0 / gramsOz
	// Buy/sell are rounded DOWN to whole baht (no fractional satang).
	return &ScrapedData{
		Buy:      math.Floor(midPerKg * buyFactor),
		Sell:     math.Floor(midPerKg * sellFactor),
		Spot:     spot,
		Exchange: thb,
		Date:     updatedAt,
	}, nil
}

func fetchSpotUSD() (price float64, updatedAt string, err error) {
	var body struct {
		Price     float64 `json:"price"`
		UpdatedAt string  `json:"updatedAt"`
	}
	if err := getJSON(xagURL, &body); err != nil {
		return 0, "", err
	}
	if body.Price <= 0 {
		return 0, "", fmt.Errorf("ราคา spot ไม่ถูกต้อง")
	}
	return body.Price, body.UpdatedAt, nil
}

func fetchUSDTHB() (float64, error) {
	var body struct {
		Rates struct {
			THB float64 `json:"THB"`
		} `json:"rates"`
	}
	if err := getJSON(fxURL, &body); err != nil {
		return 0, err
	}
	if body.Rates.THB <= 0 {
		return 0, fmt.Errorf("อัตรา THB ไม่ถูกต้อง")
	}
	return body.Rates.THB, nil
}

func getJSON(url string, out any) error {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; jk-api/1.0)")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d จาก %s", resp.StatusCode, url)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

