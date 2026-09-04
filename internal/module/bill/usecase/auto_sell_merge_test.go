package usecase

import (
	"testing"

	"jk-api/internal/entity"
	"jk-api/internal/module/bill/repository"
	notificationRepo "jk-api/internal/module/notification/repository"

	"gorm.io/gorm"
)

// fakeBillRepo implements only what CreateBill touches; the embedded interface
// makes any other call panic loudly rather than pass silently.
type fakeBillRepo struct {
	repository.BillRepository
	pending *entity.Quotation

	appendedTo    uint
	appendedItems []entity.QuotationItem
	appendedAmt   float64
	appendedOrder *uint
	created       *entity.Quotation
}

func (f *fakeBillRepo) FindPendingByCreator(uint, string, bool) (*entity.Quotation, error) {
	if f.pending == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.pending, nil
}

func (f *fakeBillRepo) AppendToBill(billID uint, items []entity.QuotationItem, amount float64, sellOrderID *uint) error {
	f.appendedTo, f.appendedItems, f.appendedAmt, f.appendedOrder = billID, items, amount, sellOrderID
	f.pending.TotalAmount += amount
	return nil
}

func (f *fakeBillRepo) Create(bill *entity.Quotation) error {
	bill.ID = 99
	f.created = bill
	return nil
}

func (f *fakeBillRepo) FindByID(id uint) (*entity.Quotation, error) {
	if f.pending != nil && f.pending.ID == id {
		return f.pending, nil
	}
	return f.created, nil
}

func (f *fakeBillRepo) GenerateCode() (string, error)      { return "BILL0002", nil }
func (f *fakeBillRepo) GenerateAdminCode() (string, error) { return "P0002", nil }

type fakeNotifRepo struct {
	notificationRepo.NotificationRepository
	sent []entity.Notification
}

func (f *fakeNotifRepo) Create(n *entity.Notification) error {
	f.sent = append(f.sent, *n)
	return nil
}

func openBill(customerID uint) *entity.Quotation {
	return &entity.Quotation{
		ID: 7, Code: "BILL0001", CreatedBy: &customerID,
		Status: repository.StatusPendingIssue, IsBill: true,
		Metal: "gold", TotalAmount: 40000,
	}
}

func goldItem(total float64) CreateBillItemRequest {
	return CreateBillItemRequest{TypeID: "1", TypeName: "ทองคำแท่ง", Metal: "gold", Weight: 1, Total: total}
}

// TestAutoSellMergesLikeAManualSell holds the rule the shop actually works by: an
// auto-sell fill is the customer's own sale, just one they did not have to be
// awake for. It lands in the bill they already have open, exactly as pressing the
// button would — the customer walks in to one bill, not two.
func TestAutoSellMergesLikeAManualSell(t *testing.T) {
	const customerID = uint(5)

	t.Run("a fill joins the customer's open bill instead of opening its own", func(t *testing.T) {
		bills := &fakeBillRepo{pending: openBill(customerID)}
		notifs := &fakeNotifRepo{}
		orderID := uint(42)

		got, err := NewBillUsecase(bills, nil, notifs).CreateBill(&CreateBillRequest{
			CreatedByUserID: customerID,
			AutoSell:        true,
			SellOrderID:     &orderID,
			Items:           []CreateBillItemRequest{goldItem(52000)},
		})
		if err != nil {
			t.Fatalf("CreateBill: %v", err)
		}

		if bills.created != nil {
			t.Fatalf("a second bill %q was opened; the fill should have joined BILL0001", bills.created.Code)
		}
		if bills.appendedTo != 7 || bills.appendedAmt != 52000 {
			t.Errorf("appended %v to bill %d, want 52000 to bill 7", bills.appendedAmt, bills.appendedTo)
		}
		if got.Code != "BILL0001" {
			t.Errorf("returned bill %q, want the open one BILL0001", got.Code)
		}
		// Without the order's link on the bill it landed in, a crash before the order
		// is marked filled leaves the recovery unable to see the fill happened.
		if bills.appendedOrder == nil || *bills.appendedOrder != orderID {
			t.Errorf("sell order link = %v, want %d", bills.appendedOrder, orderID)
		}
	})

	t.Run("the fill is announced by the engine, not by the generic sell notice", func(t *testing.T) {
		bills := &fakeBillRepo{pending: openBill(customerID)}
		notifs := &fakeNotifRepo{}
		orderID := uint(42)

		if _, err := NewBillUsecase(bills, nil, notifs).CreateBill(&CreateBillRequest{
			CreatedByUserID: customerID,
			AutoSell:        true,
			SellOrderID:     &orderID,
			Items:           []CreateBillItemRequest{goldItem(52000)},
		}); err != nil {
			t.Fatalf("CreateBill: %v", err)
		}

		// The engine's own notification names the target and the price it filled at;
		// "เพิ่มรายการขายแล้ว" on top of it reads as a second, separate sale.
		if len(notifs.sent) != 0 {
			t.Errorf("sent %d notification(s): %+v, want none from here", len(notifs.sent), notifs.sent)
		}
	})

	t.Run("a manual sell into the same bill still tells the customer", func(t *testing.T) {
		bills := &fakeBillRepo{pending: openBill(customerID)}
		notifs := &fakeNotifRepo{}

		if _, err := NewBillUsecase(bills, nil, notifs).CreateBill(&CreateBillRequest{
			CreatedByUserID: customerID,
			Items:           []CreateBillItemRequest{goldItem(52000)},
		}); err != nil {
			t.Fatalf("CreateBill: %v", err)
		}

		if len(notifs.sent) != 1 || notifs.sent[0].Type != "bill_updated" {
			t.Fatalf("notifications = %+v, want one bill_updated", notifs.sent)
		}
		if bills.appendedOrder != nil {
			t.Errorf("a manual sell stamped sell order %d on the bill", *bills.appendedOrder)
		}
	})

	t.Run("a fill still opens a bill when the customer has none", func(t *testing.T) {
		bills := &fakeBillRepo{}
		notifs := &fakeNotifRepo{}
		orderID := uint(42)

		got, err := NewBillUsecase(bills, nil, notifs).CreateBill(&CreateBillRequest{
			CreatedByUserID: customerID,
			AutoSell:        true,
			SellOrderID:     &orderID,
			Items:           []CreateBillItemRequest{goldItem(52000)},
		})
		if err != nil {
			t.Fatalf("CreateBill: %v", err)
		}

		if got.Code != "BILL0002" || !got.AutoSell || got.Status != repository.StatusPendingIssue {
			t.Errorf("new bill = %q auto_sell=%v status=%d, want BILL0002 auto_sell=true status=%d",
				got.Code, got.AutoSell, got.Status, repository.StatusPendingIssue)
		}
		if len(notifs.sent) != 0 {
			t.Errorf("sent %+v, want none — the engine announces its own fill", notifs.sent)
		}
	})
}
