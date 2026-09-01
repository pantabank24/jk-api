package usecase

import (
	"errors"
	"math"
	"strings"
	"time"

	"jk-api/internal/entity"
	"jk-api/internal/module/receipt/repository"
)

type ReceiptUsecase interface {
	GetAll(f repository.ReceiptFilter, page, limit int) ([]entity.AdminReceipt, int64, error)
	GetByID(id uint) (*entity.AdminReceipt, error)
	Create(req *ReceiptRequest, userID uint) (*entity.AdminReceipt, error)
	Update(id uint, req *ReceiptRequest) (*entity.AdminReceipt, error)
	Delete(id uint) error

	GetSettings() (*entity.ReceiptSettings, error)
	SaveSettings(req *SettingsRequest) (*entity.ReceiptSettings, error)
	SaveLogo(path string) (*entity.ReceiptSettings, error)
}

type ReceiptItemRequest struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	// Typed, not derived. The paper receipt is copied line for line, and its own
	// รวม column does not always equal จำนวน × ราคาต่อหน่วย — only the grand total
	// is added up here.
	Amount float64 `json:"amount"`
}

type ReceiptRequest struct {
	Code            string `json:"code"`
	IssuedDate      string `json:"issued_date"` // YYYY-MM-DD
	Reference       string `json:"reference"`
	CustomerName    string `json:"customer_name"`
	CustomerAddress string `json:"customer_address"`
	CustomerTaxID   string `json:"customer_tax_id"`
	PayCash         bool   `json:"pay_cash"`
	PayCheque       bool   `json:"pay_cheque"`
	// The date on the รับชำระโดย line. The bank and account number beside it are
	// defaults (ReceiptSettings), and the จำนวน is the receipt's own total, so this
	// is the only part of that block entered per receipt.
	PaidDate string               `json:"paid_date"` // YYYY-MM-DD, blank = none
	Items    []ReceiptItemRequest `json:"items"`
}

type SettingsRequest struct {
	CompanyName    string `json:"company_name"`
	CompanyAddress string `json:"company_address"`
	CompanyTaxID   string `json:"company_tax_id"`
	CompanyPhone   string `json:"company_phone"`
	DocTitle       string `json:"doc_title"`
	SellerName     string `json:"seller_name"`
	AccountName    string `json:"account_name"`
	BankName       string `json:"bank_name"`
	BankAccountNo  string `json:"bank_account_no"`
	// "" clears the logo; omitted keeps whatever is stored (logo is uploaded
	// through its own endpoint).
	LogoURL *string `json:"logo_url"`
}

type receiptUsecase struct {
	repo repository.ReceiptRepository
}

func NewReceiptUsecase(repo repository.ReceiptRepository) ReceiptUsecase {
	return &receiptUsecase{repo: repo}
}

const dateLayout = "2006-01-02"

func parseDate(s string) (time.Time, error) {
	return time.Parse(dateLayout, strings.TrimSpace(s))
}

// round2 keeps money to satang — without it the running sum picks up float noise
// and prints a figure that no longer matches its own lines.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// buildItems turns the request lines into rows and adds their รวม column up. Each
// line's amount comes in as typed; the only figure computed here is the total.
// Blank lines are dropped — the printed form has empty rows and the editor mirrors
// that, but they are not records.
func buildItems(reqItems []ReceiptItemRequest) ([]entity.AdminReceiptItem, float64) {
	items := make([]entity.AdminReceiptItem, 0, len(reqItems))
	var total float64
	for _, it := range reqItems {
		if strings.TrimSpace(it.Description) == "" && it.Quantity == 0 && it.UnitPrice == 0 && it.Amount == 0 {
			continue
		}
		amount := round2(it.Amount)
		items = append(items, entity.AdminReceiptItem{
			SortOrder:   len(items),
			Description: strings.TrimSpace(it.Description),
			Quantity:    it.Quantity,
			Unit:        strings.TrimSpace(it.Unit),
			UnitPrice:   it.UnitPrice,
			Amount:      amount,
		})
		total += amount
	}
	return items, round2(total)
}

// apply validates the request and writes it onto the receipt. Shared by create and
// update so both reject the same things and compute totals the same way.
func (u *receiptUsecase) apply(r *entity.AdminReceipt, req *ReceiptRequest) ([]entity.AdminReceiptItem, error) {
	issued, err := parseDate(req.IssuedDate)
	if err != nil {
		return nil, errors.New("กรุณาระบุวันที่ให้ถูกต้อง")
	}
	items, total := buildItems(req.Items)
	if len(items) == 0 {
		return nil, errors.New("กรุณาเพิ่มรายการอย่างน้อย 1 รายการ")
	}

	var paidDate *time.Time
	if strings.TrimSpace(req.PaidDate) != "" {
		d, err := parseDate(req.PaidDate)
		if err != nil {
			return nil, errors.New("กรุณาระบุวันที่รับชำระให้ถูกต้อง")
		}
		paidDate = &d
	}

	r.Code = strings.TrimSpace(req.Code)
	r.IssuedDate = issued
	r.Reference = strings.TrimSpace(req.Reference)
	r.CustomerName = strings.TrimSpace(req.CustomerName)
	r.CustomerAddress = req.CustomerAddress
	r.CustomerTaxID = strings.TrimSpace(req.CustomerTaxID)
	r.PayCash = req.PayCash
	r.PayCheque = req.PayCheque
	r.PaidDate = paidDate
	r.TotalAmount = total
	return items, nil
}

func (u *receiptUsecase) GetAll(f repository.ReceiptFilter, page, limit int) ([]entity.AdminReceipt, int64, error) {
	return u.repo.FindAll(f, page, limit)
}

func (u *receiptUsecase) GetByID(id uint) (*entity.AdminReceipt, error) {
	return u.repo.FindByID(id)
}

func (u *receiptUsecase) Create(req *ReceiptRequest, userID uint) (*entity.AdminReceipt, error) {
	r := &entity.AdminReceipt{}
	items, err := u.apply(r, req)
	if err != nil {
		return nil, err
	}
	if userID > 0 {
		r.CreatedBy = &userID
	}
	r.Items = items
	if err := u.repo.Create(r); err != nil {
		return nil, err
	}
	return u.repo.FindByID(r.ID)
}

func (u *receiptUsecase) Update(id uint, req *ReceiptRequest) (*entity.AdminReceipt, error) {
	r, err := u.repo.FindByID(id)
	if err != nil {
		return nil, errors.New("ไม่พบใบเสร็จ")
	}
	items, err := u.apply(r, req)
	if err != nil {
		return nil, err
	}
	// Items are rewritten by Update; leaving the preloaded ones attached would make
	// GORM try to save them again alongside the new rows.
	r.Items = nil
	if err := u.repo.Update(r, items); err != nil {
		return nil, err
	}
	return u.repo.FindByID(id)
}

func (u *receiptUsecase) Delete(id uint) error {
	if _, err := u.repo.FindByID(id); err != nil {
		return errors.New("ไม่พบใบเสร็จ")
	}
	return u.repo.Delete(id)
}

func (u *receiptUsecase) GetSettings() (*entity.ReceiptSettings, error) {
	return u.repo.GetSettings()
}

func (u *receiptUsecase) SaveSettings(req *SettingsRequest) (*entity.ReceiptSettings, error) {
	s, err := u.repo.GetSettings()
	if err != nil {
		return nil, err
	}
	s.CompanyName = strings.TrimSpace(req.CompanyName)
	s.CompanyAddress = req.CompanyAddress
	s.CompanyTaxID = strings.TrimSpace(req.CompanyTaxID)
	s.CompanyPhone = strings.TrimSpace(req.CompanyPhone)
	s.DocTitle = strings.TrimSpace(req.DocTitle)
	s.SellerName = strings.TrimSpace(req.SellerName)
	s.AccountName = strings.TrimSpace(req.AccountName)
	s.BankName = strings.TrimSpace(req.BankName)
	s.BankAccountNo = strings.TrimSpace(req.BankAccountNo)
	if req.LogoURL != nil {
		s.LogoURL = *req.LogoURL
	}
	if err := u.repo.SaveSettings(s); err != nil {
		return nil, err
	}
	return s, nil
}

func (u *receiptUsecase) SaveLogo(path string) (*entity.ReceiptSettings, error) {
	s, err := u.repo.GetSettings()
	if err != nil {
		return nil, err
	}
	s.LogoURL = path
	if err := u.repo.SaveSettings(s); err != nil {
		return nil, err
	}
	return s, nil
}
