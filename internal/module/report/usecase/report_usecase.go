package usecase

import (
	"jk-api/internal/module/report/repository"
)

// SalesReport is everything the page draws above its table: the headline totals,
// the chart series, and the two breakdowns. The document rows are fetched
// separately because they page and these do not.
type SalesReport struct {
	Metal      string                      `json:"metal"`
	Overview   repository.SalesOverview    `json:"overview"`
	Series     []repository.SalesPoint     `json:"series"`
	ByType     []repository.SalesBreakdown `json:"by_type"`
	ByEmployee []repository.SalesBreakdown `json:"by_employee"`
}

type ReportUsecase interface {
	Sales(f repository.SalesFilter) (*SalesReport, error)
	Rows(f repository.SalesFilter, page, limit int) ([]repository.SalesRow, int64, error)
	ExportData(f repository.SalesFilter) (*SalesReport, []repository.SalesRow, []repository.SalesItemRow, error)
}

type reportUsecase struct {
	repo repository.ReportRepository
}

func NewReportUsecase(repo repository.ReportRepository) ReportUsecase {
	return &reportUsecase{repo: repo}
}

func (u *reportUsecase) Sales(f repository.SalesFilter) (*SalesReport, error) {
	overview, err := u.repo.Overview(f)
	if err != nil {
		return nil, err
	}
	series, err := u.repo.Series(f)
	if err != nil {
		return nil, err
	}
	byType, err := u.repo.ByType(f)
	if err != nil {
		return nil, err
	}
	byEmployee, err := u.repo.ByEmployee(f)
	if err != nil {
		return nil, err
	}
	return &SalesReport{
		Metal:      f.Metal,
		Overview:   overview,
		Series:     emptyIfNil(series),
		ByType:     emptyIfNil(byType),
		ByEmployee: emptyIfNil(byEmployee),
	}, nil
}

func (u *reportUsecase) Rows(f repository.SalesFilter, page, limit int) ([]repository.SalesRow, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = 50
	}
	return u.repo.Rows(f, page, limit)
}

// ExportData gathers the whole filtered set — summary, one line per document,
// and one line per item — for the workbook's three sheets.
func (u *reportUsecase) ExportData(f repository.SalesFilter) (*SalesReport, []repository.SalesRow, []repository.SalesItemRow, error) {
	report, err := u.Sales(f)
	if err != nil {
		return nil, nil, nil, err
	}
	rows, err := u.repo.AllRows(f)
	if err != nil {
		return nil, nil, nil, err
	}
	items, err := u.repo.ItemRows(f)
	if err != nil {
		return nil, nil, nil, err
	}
	return report, rows, items, nil
}

// emptyIfNil keeps the JSON an array rather than null, so the page can map over
// it without a guard on every field.
func emptyIfNil[T any](in []T) []T {
	if in == nil {
		return []T{}
	}
	return in
}
