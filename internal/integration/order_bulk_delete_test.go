package integration_test

import (
	"net/http"
	"testing"

	"almak-back/internal/models"
)

func TestBulkDeleteOrdersIsAtomicAndCascades(t *testing.T) {
	a := setupDraftAPI(t, "")
	var ids []uint
	for range 3 {
		order := models.Order{
			Customer: "Test", Phone: "79000000000", Date: "2026-11-07", Status: "accepted",
			Payments:      []models.OrderPayment{{Amount: 100}},
			InteriorDoors: []models.InteriorDoor{{Model: "Test", Width: 80, Height: 200, Count: 1}},
		}
		if err := a.db.Create(&order).Error; err != nil {
			t.Fatal(err)
		}
		ids = append(ids, order.ID)
	}
	path := "/orders/bulk-delete"
	body := map[string]any{"ids": ids[:2]}
	if res := a.request(-1, "POST", path, body); res.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous delete: %d", res.Code)
	}
	if res := a.request(1, "POST", path, map[string]any{"ids": []uint{ids[0], 0}}); res.Code != http.StatusBadRequest {
		t.Fatalf("invalid ids: %d", res.Code)
	}
	if err := a.db.Exec("CREATE TABLE test_delete_blocker (order_id bigint REFERENCES orders(id) ON DELETE RESTRICT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := a.db.Exec("INSERT INTO test_delete_blocker VALUES (?)", ids[1]).Error; err != nil {
		t.Fatal(err)
	}
	if res := a.request(1, "POST", path, body); res.Code != http.StatusInternalServerError {
		t.Fatalf("expected rollback: %d", res.Code)
	}

	check := func(want int64) {
		t.Helper()
		for _, model := range []any{&models.Order{}, &models.OrderPayment{}, &models.InteriorDoor{}} {
			var count int64
			if err := a.db.Model(model).Count(&count).Error; err != nil || count != want {
				t.Fatalf("%T: got %d, want %d, error: %v", model, count, want, err)
			}
		}
	}
	check(3)
	if err := a.db.Exec("DELETE FROM test_delete_blocker").Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if res := a.request(1, "POST", path, body); res.Code != http.StatusNoContent {
			t.Fatalf("bulk delete: %d %s", res.Code, res.Body.String())
		}
	}
	check(1)
	var remaining models.Order
	if err := a.db.First(&remaining).Error; err != nil || remaining.ID != ids[2] {
		t.Fatal("an unselected order was deleted")
	}
}
