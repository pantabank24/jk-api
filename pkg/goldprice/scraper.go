package goldprice

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

type ScrapedData struct {
	BarBuy          float64
	BarSell         float64
	OrnamentBuy     float64
	OrnamentSell    float64
	ChangeToday     float64
	ChangeYesterday float64
	GoldDate        string
	GoldTime        string
	GoldRound       string
}

var thaiMonths = []string{
	"มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน",
	"พฤษภาคม", "มิถุนายน", "กรกฎาคม", "สิงหาคม",
	"กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม",
}

func Fetch() (*ScrapedData, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", "https://xn--42cah7d0cxcvbbb9x.com/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36")
	req.Header.Set("Accept-Language", "th-TH,th;q=0.9,en-US;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ไม่สามารถเชื่อมต่อได้: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML failed: %w", err)
	}

	data := &ScrapedData{}

	doc.Find("tr").Each(func(_ int, s *goquery.Selection) {
		tds := s.Find("td")
		if tds.Length() < 2 {
			return
		}
		first := strings.TrimSpace(tds.Eq(0).Text())

		if strings.Contains(first, "ทองคำแท่ง") {
			if tds.Length() >= 3 {
				data.BarSell = parsePrice(tds.Eq(1).Text())
				data.BarBuy = parsePrice(tds.Eq(2).Text())
			}
		}

		if tds.Length() == 3 && strings.Contains(first, "ทองรูปพรรณ") {
			data.OrnamentSell = parsePrice(tds.Eq(1).Text())
			data.OrnamentBuy = parsePrice(tds.Eq(2).Text())
		}

		if strings.Contains(first, "วันนี้") {
			data.ChangeToday = parseChangePrice(first)
			if tds.Length() >= 3 {
				data.ChangeYesterday = parsePrice(tds.Eq(2).Text())
			}
		}

		for _, month := range thaiMonths {
			if strings.Contains(first, month) {
				data.GoldDate = first
				if tds.Length() >= 2 {
					data.GoldTime = strings.TrimSpace(tds.Eq(1).Text())
				}
				if tds.Length() >= 3 {
					data.GoldRound = strings.TrimSpace(tds.Eq(2).Text())
				}
				break
			}
		}
	})

	if data.BarBuy == 0 && data.BarSell == 0 {
		return nil, fmt.Errorf("ไม่พบข้อมูลราคาทอง")
	}
	return data, nil
}

func parsePrice(s string) float64 {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	var result float64
	fmt.Sscanf(s, "%f", &result)
	return result
}

func parseChangePrice(s string) float64 {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) || r == '.' {
			b.WriteRune(r)
		}
	}
	// check for minus sign
	if strings.Contains(s, "-") || strings.Contains(s, "ลด") {
		var v float64
		fmt.Sscanf(b.String(), "%f", &v)
		return -v
	}
	var v float64
	fmt.Sscanf(b.String(), "%f", &v)
	return v
}
