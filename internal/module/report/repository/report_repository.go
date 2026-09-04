package repository

import (
	"time"

	"gorm.io/gorm"
)

// รายงานยอดขาย — one metal at a time, over the documents the shop considers
// FINISHED. "Finished" is deliberately two different things, because money
// arrives by two different routes:
//
//   - a quotation issued against customer bills is finished when those bills have
//     been settled (เคลียร์แล้ว, status 14) — until then the shop still owes for it;
//   - a walk-in quotation has no bill behind it at all, so it is finished the
//     moment it is issued (quotations are created at status 1).
//
// Both live in the same `quotations` table, so one NOT EXISTS covers both: a
// document counts unless some bill points at it that has not been cleared.
const (
	quotationStatusIssued = 1
	billStatusCleared     = 14
)

// SalesFilter is the whole filter bar. Store/Branch/CreatedBy are resolved from
// the caller's token in the controller (a master may narrow them by query), the
// rest are the user's own choices.
type SalesFilter struct {
	Metal     string
	From      *time.Time
	To        *time.Time
	StoreID   *uint
	BranchID  *uint
	CreatedBy *uint
	// TypeID narrows to one gold type (ทอง 96.5, เงิน 99.9, …).
	TypeID string
	// Customer matches the signer or the linked member's name.
	Customer string
	// Search matches the document number — the quotation's own code, or the bill
	// code it is displayed under.
	Search string
	// Bucket is the chart's x-axis: day | month.
	Bucket string
}

type SalesOverview struct {
	Amount     float64 `json:"amount"`
	Weight     float64 `json:"weight"`
	DocCount   int64   `json:"doc_count"`
	ItemCount  int64   `json:"item_count"`
	AvgPerGram float64 `json:"avg_per_gram"`
}

type SalesPoint struct {
	Bucket string  `json:"bucket"`
	Amount float64 `json:"amount"`
	Weight float64 `json:"weight"`
	Docs   int64   `json:"docs"`
}

// SalesBreakdown is one slice of the pie — by gold type, by employee, by branch.
type SalesBreakdown struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Amount float64 `json:"amount"`
	Weight float64 `json:"weight"`
	Docs   int64   `json:"docs"`
}

// SalesRow is one document in the report's table. Weight/Amount are THIS metal's
// share of the document, not the document's grand total: a walk-in quotation may
// mix metals, and the ทอง report must not claim its เงิน lines.
type SalesRow struct {
	QuotationID uint      `json:"quotation_id"`
	Code        string    `json:"code"`
	CreatedAt   time.Time `json:"created_at"`
	Customer    string    `json:"customer"`
	Phone       string    `json:"phone"`
	Employee    string    `json:"employee"`
	Branch      string    `json:"branch"`
	// Source is "bill" (issued against a customer's bill) or "walkin".
	Source string  `json:"source"`
	Weight float64 `json:"weight"`
	Amount float64 `json:"amount"`
}

// SalesItemRow is the line-level detail — only used by the Excel export's second
// sheet, where an accountant wants to see each ประเภททอง rather than a total.
type SalesItemRow struct {
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	Customer  string    `json:"customer"`
	Employee  string    `json:"employee"`
	TypeName  string    `json:"type_name"`
	Percent   float64   `json:"percent"`
	Price     float64   `json:"price"`
	Weight    float64   `json:"weight"`
	PerGram   float64   `json:"per_gram"`
	Amount    float64   `json:"amount"`
}

type ReportRepository interface {
	Overview(f SalesFilter) (SalesOverview, error)
	Series(f SalesFilter) ([]SalesPoint, error)
	ByType(f SalesFilter) ([]SalesBreakdown, error)
	ByEmployee(f SalesFilter) ([]SalesBreakdown, error)
	Rows(f SalesFilter, page, limit int) ([]SalesRow, int64, error)
	AllRows(f SalesFilter) ([]SalesRow, error)
	ItemRows(f SalesFilter) ([]SalesItemRow, error)
}

type reportRepository struct {
	db *gorm.DB
}

func NewReportRepository(db *gorm.DB) ReportRepository {
	return &reportRepository{db: db}
}

// displayCodeExpr mirrors the rule the lists and printed documents already use:
// a quotation issued from a bill is known by the BILL's number, not its own.
const displayCodeExpr = `COALESCE((
	SELECT MIN(b.code) FROM quotations b
	WHERE b.is_bill = TRUE AND b.deleted_at IS NULL AND b.issued_quotation_id = q.id
), q.code)`

// sourceExpr labels where the document came from, so a mixed report still reads.
const sourceExpr = `CASE WHEN EXISTS (
	SELECT 1 FROM quotations b
	WHERE b.is_bill = TRUE AND b.deleted_at IS NULL AND b.issued_quotation_id = q.id
) THEN 'bill' ELSE 'walkin' END`

// customerExpr is who the document was for: the linked member if there is one,
// otherwise the name signed on the paper.
const customerExpr = `COALESCE(NULLIF(TRIM(CONCAT(m.fname, ' ', m.lname)), ''), NULLIF(q.signer_name, ''), '')`

// scope builds the filtered item set. Every aggregate below starts here, so the
// chart, the totals, the table and the export can never disagree.
func (r *reportRepository) scope(f SalesFilter) *gorm.DB {
	q := r.db.Table("quotation_items AS i").
		Joins("JOIN quotations q ON q.id = i.quotation_id AND q.deleted_at IS NULL").
		Joins("LEFT JOIN users u ON u.id = q.created_by").
		Joins("LEFT JOIN branches br ON br.id = q.branch_id").
		Joins("LEFT JOIN members m ON m.id = q.member_id AND m.deleted_at IS NULL").
		Where("i.deleted_at IS NULL").
		Where("q.is_bill = ?", false).
		Where("q.status = ?", quotationStatusIssued).
		Where(`NOT EXISTS (
			SELECT 1 FROM quotations b
			WHERE b.is_bill = TRUE AND b.deleted_at IS NULL
			  AND b.issued_quotation_id = q.id AND b.status <> ?
		)`, billStatusCleared).
		// Items carry their own metal tag; rows that predate the column are gold
		// by construction (migration 000071), so the COALESCE is exact, not a guess.
		Where("COALESCE(NULLIF(i.metal, ''), 'gold') = ?", f.Metal)

	if f.From != nil {
		q = q.Where("q.created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("q.created_at < ?", *f.To)
	}
	if f.StoreID != nil {
		q = q.Where("q.store_id = ?", *f.StoreID)
	}
	if f.BranchID != nil {
		q = q.Where("q.branch_id = ?", *f.BranchID)
	}
	if f.CreatedBy != nil {
		q = q.Where("q.created_by = ?", *f.CreatedBy)
	}
	if f.TypeID != "" {
		q = q.Where("i.type_id = ?", f.TypeID)
	}
	if f.Customer != "" {
		like := "%" + f.Customer + "%"
		q = q.Where("("+customerExpr+" ILIKE ? OR q.signer_phone ILIKE ?)", like, like)
	}
	if f.Search != "" {
		like := "%" + f.Search + "%"
		q = q.Where("(q.code ILIKE ? OR "+displayCodeExpr+" ILIKE ? OR "+customerExpr+" ILIKE ?)", like, like, like)
	}
	return q
}

func (r *reportRepository) Overview(f SalesFilter) (SalesOverview, error) {
	var out SalesOverview
	err := r.scope(f).Select(`
		COALESCE(SUM(i.total), 0)            AS amount,
		COALESCE(SUM(i.weight), 0)           AS weight,
		COUNT(DISTINCT q.id)                 AS doc_count,
		COUNT(i.id)                          AS item_count`).
		Scan(&out).Error
	if err == nil && out.Weight > 0 {
		out.AvgPerGram = out.Amount / out.Weight
	}
	return out, err
}

// bucketExpr groups by the shop's calendar, not the server's: a sale keyed in at
// 00:30 in Bangkok belongs to that day, whatever UTC calls it.
func bucketExpr(bucket string) string {
	if bucket == "month" {
		return `to_char(q.created_at AT TIME ZONE 'Asia/Bangkok', 'YYYY-MM')`
	}
	return `to_char(q.created_at AT TIME ZONE 'Asia/Bangkok', 'YYYY-MM-DD')`
}

func (r *reportRepository) Series(f SalesFilter) ([]SalesPoint, error) {
	expr := bucketExpr(f.Bucket)
	var out []SalesPoint
	err := r.scope(f).
		Select(expr + ` AS bucket,
			COALESCE(SUM(i.total), 0)  AS amount,
			COALESCE(SUM(i.weight), 0) AS weight,
			COUNT(DISTINCT q.id)       AS docs`).
		Group(expr).Order(expr + " ASC").Scan(&out).Error
	return out, err
}

func (r *reportRepository) ByType(f SalesFilter) ([]SalesBreakdown, error) {
	var out []SalesBreakdown
	err := r.scope(f).
		Select(`i.type_id AS key, MIN(i.type_name) AS label,
			COALESCE(SUM(i.total), 0)  AS amount,
			COALESCE(SUM(i.weight), 0) AS weight,
			COUNT(DISTINCT q.id)       AS docs`).
		Group("i.type_id").Order("amount DESC").Scan(&out).Error
	return out, err
}

func (r *reportRepository) ByEmployee(f SalesFilter) ([]SalesBreakdown, error) {
	var out []SalesBreakdown
	err := r.scope(f).
		Select(`COALESCE(q.created_by::text, '') AS key,
			COALESCE(MIN(u.name), 'ไม่ระบุ')      AS label,
			COALESCE(SUM(i.total), 0)             AS amount,
			COALESCE(SUM(i.weight), 0)            AS weight,
			COUNT(DISTINCT q.id)                  AS docs`).
		Group("q.created_by").Order("amount DESC").Scan(&out).Error
	return out, err
}

// rowsQuery collapses the item set back to one line per document, keeping only
// this metal's share of it.
func (r *reportRepository) rowsQuery(f SalesFilter) *gorm.DB {
	return r.scope(f).
		Select(`q.id AS quotation_id,
			` + displayCodeExpr + ` AS code,
			q.created_at            AS created_at,
			` + customerExpr + `    AS customer,
			COALESCE(q.signer_phone, '') AS phone,
			COALESCE(u.name, '')    AS employee,
			COALESCE(br.name, '')   AS branch,
			` + sourceExpr + `      AS source,
			COALESCE(SUM(i.weight), 0) AS weight,
			COALESCE(SUM(i.total), 0)  AS amount`).
		Group("q.id, q.created_at, q.signer_name, q.signer_phone, q.code, m.fname, m.lname, u.name, br.name")
}

func (r *reportRepository) Rows(f SalesFilter, page, limit int) ([]SalesRow, int64, error) {
	var rows []SalesRow

	// Count the documents, not the item rows the aggregate is built from.
	var total int64
	if err := r.scope(f).Distinct("q.id").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.rowsQuery(f).
		Order("q.created_at DESC, q.id DESC").
		Offset((page - 1) * limit).Limit(limit).
		Scan(&rows).Error
	return rows, total, err
}

func (r *reportRepository) AllRows(f SalesFilter) ([]SalesRow, error) {
	var rows []SalesRow
	err := r.rowsQuery(f).Order("q.created_at ASC, q.id ASC").Scan(&rows).Error
	return rows, err
}

func (r *reportRepository) ItemRows(f SalesFilter) ([]SalesItemRow, error) {
	var rows []SalesItemRow
	err := r.scope(f).
		Select(`` + displayCodeExpr + ` AS code,
			q.created_at         AS created_at,
			` + customerExpr + ` AS customer,
			COALESCE(u.name, '') AS employee,
			i.type_name          AS type_name,
			i.percent            AS percent,
			i.price              AS price,
			i.weight             AS weight,
			i.per_gram           AS per_gram,
			i.total              AS amount`).
		Order("q.created_at ASC, q.id ASC, i.id ASC").Scan(&rows).Error
	return rows, err
}
