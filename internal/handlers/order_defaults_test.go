package handlers

import (
	"almak-back/internal/aiorders"
	"testing"
	"time"
)

func TestOrderDefaultsUseMoscowDateAndPreserveExplicitValues(t *testing.T) {
	now := time.Date(2026, 11, 8, 22, 30, 0, 0, time.UTC)
	order := aiorders.Order{Header: map[string]any{}, Items: []aiorders.Item{{Type: "hardware", Fields: map[string]any{"lockCount": float64(3), "escutcheonCount": float64(0)}}}}
	applyOrderDefaults("initial", aiorders.Order{}, &order, now)
	if order.Header["date"] != "2026-11-09" || order.Items[0].Fields["cylinderCount"] != float64(3) || order.Items[0].Fields["escutcheonCount"] != float64(0) {
		t.Fatal(order)
	}
	order.Header["date"] = "2026-10-01"
	applyOrderDefaults("refine", aiorders.Order{}, &order, now)
	if order.Header["date"] != "2026-10-01" {
		t.Fatal(order)
	}
	order.Header["date"] = nil
	applyOrderDefaults("edit_order", aiorders.Order{}, &order, now)
	if order.Header["date"] != nil {
		t.Fatal("editing changed the existing date")
	}
}

func TestEditingUnrelatedFieldsDoesNotAddHardwareAccessories(t *testing.T) {
	before := aiorders.Order{Header: map[string]any{"phone": "79000000000"}, Items: []aiorders.Item{{ItemID: "hardware-1", Type: "hardware", Fields: map[string]any{"lockCount": float64(3)}}}}
	after := aiorders.Order{Header: map[string]any{"phone": "79000000001"}, Items: []aiorders.Item{{ItemID: "hardware-1", Type: "hardware", Fields: map[string]any{"lockCount": float64(3)}}}}
	applyOrderDefaults("edit_order", before, &after, time.Now())
	if after.Items[0].Fields["cylinderCount"] != nil || after.Items[0].Fields["escutcheonCount"] != nil {
		t.Fatal(after)
	}
}
