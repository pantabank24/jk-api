package controller

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jk-api/internal/middleware"
	"jk-api/internal/module/report/repository"
	"jk-api/internal/module/report/usecase"
	"jk-api/internal/service"
	"jk-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type ReportController struct {
	uc usecase.ReportUsecase
	db *gorm.DB
}

func NewReportController(uc usecase.ReportUsecase, db *gorm.DB) *ReportController {
	return &ReportController{uc: uc, db: db}
}

// metalLabel is how each metal is named in Thai on the exported sheet.
var metalLabel = map[string]string{
	"gold":      "ทอง",
	"silver":    "เงิน",
	"platinum":  "แพลตินัม",
	"palladium": "แพลเลเดียม",
}

func isReportMetal(metal string) bool {
	_, ok := metalLabel[metal]
	return ok
}

// buildFilter reads the filter bar off the query string and then OVERRIDES the
// scope fields from the token. Reporting reuses the quotations.read scope rules
// exactly — an employee's report covers their own documents, an owner's covers
// their store, a master's covers whatever they narrow to — so the report can
// never show more than the list it is summarising.
func (ctrl *ReportController) buildFilter(c *fiber.Ctx) (repository.SalesFilter, error) {
	f := repository.SalesFilter{
		Metal:    strings.TrimSpace(c.Query("metal", "gold")),
		TypeID:   strings.TrimSpace(c.Query("type_id")),
		Customer: strings.TrimSpace(c.Query("customer")),
		Search:   strings.TrimSpace(c.Query("search")),
		Bucket:   strings.TrimSpace(c.Query("bucket", "day")),
	}
	if !isReportMetal(f.Metal) {
		return f, fmt.Errorf("metal ไม่ถูกต้อง")
	}
	if f.Bucket != "month" {
		f.Bucket = "day"
	}

	loc := service.BangkokLocation()
	if v := c.Query("date_from"); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, loc)
		if err != nil {
			return f, fmt.Errorf("date_from ไม่ถูกต้อง")
		}
		f.From = &t
	}
	if v := c.Query("date_to"); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, loc)
		if err != nil {
			return f, fmt.Errorf("date_to ไม่ถูกต้อง")
		}
		// Inclusive end date — the scope compares with "<", so push to the next midnight.
		end := t.AddDate(0, 0, 1)
		f.To = &end
	}

	if middleware.IsMaster(c) {
		f.StoreID = queryUint(c, "store_id")
		f.BranchID = queryUint(c, "branch_id")
		f.CreatedBy = queryUint(c, "created_by")
		return f, nil
	}

	f.StoreID = middleware.GetStoreID(c)
	switch middleware.GetRoleName(c) {
	case "owner":
		// Owner covers the whole store and may narrow to a branch or a person.
		f.BranchID = queryUint(c, "branch_id")
		f.CreatedBy = queryUint(c, "created_by")
	default:
		// Employee: their own branch, their own documents. Not negotiable by query.
		f.BranchID = middleware.GetBranchID(c)
		userID := middleware.GetUserID(c)
		f.CreatedBy = &userID
	}
	return f, nil
}

func queryUint(c *fiber.Ctx, key string) *uint {
	v := c.Query(key)
	if v == "" {
		return nil
	}
	id, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return nil
	}
	out := uint(id)
	return &out
}

func (ctrl *ReportController) GetSales(c *fiber.Ctx) error {
	f, err := ctrl.buildFilter(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	report, err := ctrl.uc.Sales(f)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, "Sales report", report)
}

func (ctrl *ReportController) GetSalesRows(c *fiber.Ctx) error {
	f, err := ctrl.buildFilter(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	rows, total, err := ctrl.uc.Rows(f, page, limit)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = 50
	}
	return response.Paginated(c, "Sales rows", rows, page, limit, total)
}

// ExportSales writes the filtered report as a real .xlsx: a summary sheet that
// reads like the page's header, one row per document, and one row per item for
// whoever has to reconcile it line by line.
func (ctrl *ReportController) ExportSales(c *fiber.Ctx) error {
	f, err := ctrl.buildFilter(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	report, rows, items, err := ctrl.uc.ExportData(f)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	file := excelize.NewFile()
	defer file.Close()

	head, err := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"C09C42"}},
	})
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	money, err := file.NewStyle(&excelize.Style{CustomNumFmt: strPtr("#,##0.00")})
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	gram, err := file.NewStyle(&excelize.Style{CustomNumFmt: strPtr("#,##0.0000")})
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	if err := ctrl.writeSummarySheet(file, f, report, head, money, gram); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	if err := writeDocumentSheet(file, rows, head, money, gram); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	if err := writeItemSheet(file, items, head, money, gram); err != nil {
		return response.InternalServerError(c, err.Error())
	}

	// Sheet1 is excelize's default empty sheet; the real first sheet replaced it.
	file.SetActiveSheet(0)

	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		return response.InternalServerError(c, err.Error())
	}

	name := fmt.Sprintf("sales_%s_%s.xlsx", f.Metal, time.Now().In(service.BangkokLocation()).Format("20060102_1504"))
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", "attachment; filename="+name)
	return c.Send(buf.Bytes())
}

func strPtr(s string) *string { return &s }

const (
	sheetSummary  = "สรุป"
	sheetDocument = "รายเอกสาร"
	sheetItem     = "รายรายการ"
)

func (ctrl *ReportController) writeSummarySheet(
	file *excelize.File, f repository.SalesFilter, report *usecase.SalesReport,
	head, money, gram int,
) error {
	// Rename the default sheet rather than adding a fourth empty one.
	if err := file.SetSheetName("Sheet1", sheetSummary); err != nil {
		return err
	}
	s := sheetSummary
	loc := service.BangkokLocation()
	period := "ทั้งหมด"
	if f.From != nil || f.To != nil {
		from, to := "…", "…"
		if f.From != nil {
			from = f.From.In(loc).Format("02/01/2006")
		}
		if f.To != nil {
			// To is the exclusive next midnight; show the day the user picked.
			to = f.To.In(loc).AddDate(0, 0, -1).Format("02/01/2006")
		}
		period = from + " - " + to
	}

	rows := [][]any{
		{"รายงานยอดขาย" + metalLabel[f.Metal]},
		{"ช่วงวันที่", period},
		{"ออกรายงานเมื่อ", time.Now().In(loc).Format("02/01/2006 15:04")},
		{},
		{"ยอดรวม (บาท)", report.Overview.Amount},
		{"น้ำหนักรวม (กรัม)", report.Overview.Weight},
		{"จำนวนเอกสาร", report.Overview.DocCount},
		{"จำนวนรายการ", report.Overview.ItemCount},
		{"ราคาเฉลี่ยต่อกรัม", report.Overview.AvgPerGram},
	}
	for i, row := range rows {
		if err := file.SetSheetRow(s, cell("A", i+1), &row); err != nil {
			return err
		}
	}
	_ = file.SetCellStyle(s, "B5", "B6", money)
	_ = file.SetCellStyle(s, "B9", "B9", money)

	// ── แยกตามประเภท ──
	line := len(rows) + 2
	_ = file.SetSheetRow(s, cell("A", line), &[]any{"แยกตามประเภท"})
	line++
	header := []any{"ประเภท", "น้ำหนัก (กรัม)", "ยอดรวม (บาท)", "จำนวนเอกสาร"}
	_ = file.SetSheetRow(s, cell("A", line), &header)
	_ = file.SetCellStyle(s, cell("A", line), cell("D", line), head)
	first := line + 1
	for _, b := range report.ByType {
		line++
		_ = file.SetSheetRow(s, cell("A", line), &[]any{b.Label, b.Weight, b.Amount, b.Docs})
	}
	if line >= first {
		_ = file.SetCellStyle(s, cell("B", first), cell("B", line), gram)
		_ = file.SetCellStyle(s, cell("C", first), cell("C", line), money)
	}

	// ── แยกตามพนักงาน ──
	line += 2
	_ = file.SetSheetRow(s, cell("A", line), &[]any{"แยกตามพนักงาน"})
	line++
	header = []any{"พนักงาน", "น้ำหนัก (กรัม)", "ยอดรวม (บาท)", "จำนวนเอกสาร"}
	_ = file.SetSheetRow(s, cell("A", line), &header)
	_ = file.SetCellStyle(s, cell("A", line), cell("D", line), head)
	first = line + 1
	for _, b := range report.ByEmployee {
		line++
		_ = file.SetSheetRow(s, cell("A", line), &[]any{b.Label, b.Weight, b.Amount, b.Docs})
	}
	if line >= first {
		_ = file.SetCellStyle(s, cell("B", first), cell("B", line), gram)
		_ = file.SetCellStyle(s, cell("C", first), cell("C", line), money)
	}

	_ = file.SetColWidth(s, "A", "A", 26)
	_ = file.SetColWidth(s, "B", "D", 18)
	return nil
}

func writeDocumentSheet(file *excelize.File, rows []repository.SalesRow, head, money, gram int) error {
	if _, err := file.NewSheet(sheetDocument); err != nil {
		return err
	}
	s := sheetDocument
	loc := service.BangkokLocation()
	header := []any{"เลขที่เอกสาร", "วันที่", "ลูกค้า", "เบอร์โทร", "พนักงาน", "สาขา", "ที่มา", "น้ำหนัก (กรัม)", "ยอดรวม (บาท)"}
	if err := file.SetSheetRow(s, "A1", &header); err != nil {
		return err
	}
	_ = file.SetCellStyle(s, "A1", "I1", head)

	for i, r := range rows {
		line := i + 2
		_ = file.SetSheetRow(s, cell("A", line), &[]any{
			r.Code,
			r.CreatedAt.In(loc).Format("02/01/2006 15:04"),
			r.Customer,
			r.Phone,
			r.Employee,
			r.Branch,
			sourceLabel(r.Source),
			r.Weight,
			r.Amount,
		})
	}
	if len(rows) > 0 {
		_ = file.SetCellStyle(s, "H2", cell("H", len(rows)+1), gram)
		_ = file.SetCellStyle(s, "I2", cell("I", len(rows)+1), money)
	}
	_ = file.SetColWidth(s, "A", "A", 18)
	_ = file.SetColWidth(s, "B", "G", 16)
	_ = file.SetColWidth(s, "H", "I", 16)
	return file.AutoFilter(s, "A1:I1", []excelize.AutoFilterOptions{})
}

func writeItemSheet(file *excelize.File, items []repository.SalesItemRow, head, money, gram int) error {
	if _, err := file.NewSheet(sheetItem); err != nil {
		return err
	}
	s := sheetItem
	loc := service.BangkokLocation()
	header := []any{"เลขที่เอกสาร", "วันที่", "ลูกค้า", "พนักงาน", "ประเภท", "เปอร์เซ็นต์", "ราคา", "น้ำหนัก (กรัม)", "ราคา/กรัม", "รวม (บาท)"}
	if err := file.SetSheetRow(s, "A1", &header); err != nil {
		return err
	}
	_ = file.SetCellStyle(s, "A1", "J1", head)

	for i, it := range items {
		line := i + 2
		_ = file.SetSheetRow(s, cell("A", line), &[]any{
			it.Code,
			it.CreatedAt.In(loc).Format("02/01/2006 15:04"),
			it.Customer,
			it.Employee,
			it.TypeName,
			it.Percent,
			it.Price,
			it.Weight,
			it.PerGram,
			it.Amount,
		})
	}
	if len(items) > 0 {
		last := len(items) + 1
		_ = file.SetCellStyle(s, "G2", cell("G", last), money)
		_ = file.SetCellStyle(s, "H2", cell("H", last), gram)
		_ = file.SetCellStyle(s, "I2", cell("I", last), money)
		_ = file.SetCellStyle(s, "J2", cell("J", last), money)
	}
	_ = file.SetColWidth(s, "A", "A", 18)
	_ = file.SetColWidth(s, "B", "J", 16)
	return file.AutoFilter(s, "A1:J1", []excelize.AutoFilterOptions{})
}

func sourceLabel(source string) string {
	if source == "bill" {
		return "บิลลูกค้า"
	}
	return "เดินเข้า"
}

func cell(col string, row int) string {
	return col + strconv.Itoa(row)
}
