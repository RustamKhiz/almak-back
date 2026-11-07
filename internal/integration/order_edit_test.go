package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"almak-back/internal/aiorders"
	"almak-back/internal/models"
)

func TestOrderAIEditPreviewApplyAndRevisionConflict(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var call struct{ Messages []struct{ Content string } }
		if err := json.NewDecoder(r.Body).Decode(&call); err != nil || len(call.Messages) != 2 {
			w.WriteHeader(400)
			return
		}
		var input struct{ Request aiorders.Request }
		if err := json.Unmarshal([]byte(call.Messages[1].Content), &input); err != nil || input.Request.Mode != "edit_order" {
			w.WriteHeader(400)
			return
		}
		proposal := input.Request.CurrentDraft
		if proposal.Header["prepayment"] != nil || proposal.Header["discount"] != nil ||
			proposal.Items[0].Fields["costPrice"] != nil || proposal.Items[0].Fields["supplier"] != nil {
			w.WriteHeader(400)
			return
		}
		first := &proposal.Items[0]
		first.Fields["width"] = float64(70)
		first.Fields["costPrice"] = float64(1) // Ignore protected fields even if the provider returns them.
		proposal.Header["discount"] = float64(0)
		copyFields := map[string]any{}
		for key, value := range first.Fields {
			copyFields[key] = value
		}
		proposal.Items = append(proposal.Items[:1], proposal.Items[2:]...) // Remove the second door.
		proposal.Items = append(proposal.Items, aiorders.Item{Type: "interiorDoor", ItemID: "added-door", Fields: copyFields})
		data, _ := json.Marshal(aiorders.Result{Order: proposal, Questions: []string{}})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{
			{"finish_reason": "stop", "message": map[string]any{"content": string(data)}},
		}})
	}))
	defer provider.Close()
	a := setupDraftAPI(t, provider.URL)
	width2, count2, price2, height2 := 40, 1, 450.0, 200
	order := models.Order{
		Customer: "Test customer", Phone: "79000000000", Date: "2026-11-01",
		Status: "ordered", Discount: 300, Prepayment: 500, Price: 4000,
		InteriorDoors: []models.InteriorDoor{{
			Model: "Verona", Color: "White", Covering: "Enamel", LeafType: "Double",
			Width: 80, Height: 200, Count: 1, Price: 1000, Width2: &width2, Height2: &height2, Count2: &count2, Price2: &price2,
			Supplier: "Test supplier", CostPrice: 700,
		}, {Model: "Remove me", Width: 90, Height: 200, Count: 1}},
		Panelings: []models.Paneling{{
			Width: 20, Height: 30, Count: 2, Price: 100, Color: "White", Covering: "Enamel", Kind: "smooth",
			Size: "20x30; 40x50", Sizes: models.Sizes{{Width: 20, Height: 30}, {Width: 40, Height: 50}}, TotalArea: 0.26,
		}},
		Payments: []models.OrderPayment{{Amount: 500}},
	}
	if err := a.db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/orders/%d", order.ID)
	open := func(actor int, id string) models.OrderDraft {
		return readRow(t, a.request(actor, "POST", path+"/ai-edit", map[string]any{"id": id}))
	}
	first := open(0, draftOne)
	if reopened := open(0, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"); reopened.ID != first.ID {
		t.Fatal("same editor did not reuse working copy")
	}
	second := open(1, draftTwo)
	if denied := a.request(1, "GET", "/order-drafts/"+first.ID, nil); denied.Code != 404 {
		t.Fatalf("working copy leaked: %d", denied.Code)
	}
	send := func(actor int, row models.OrderDraft) models.OrderDraft {
		return readRow(t, a.request(actor, "POST", "/order-drafts/"+row.ID+"/messages", map[string]any{
			"requestId":    fmt.Sprintf("00000000-0000-4000-8000-%012d", actor+1),
			"baseRevision": row.Revision, "text": "Change first width, remove the second door and duplicate the first",
		}))
	}
	first, second = send(0, first), send(1, second)
	foundWidth := false
	for _, change := range first.Document.Changes {
		if change.Before == float64(80) && change.After == float64(70) {
			foundWidth = true
		}
		if change.Path == "header.discount" {
			t.Fatal("protected discount appeared in preview")
		}
	}
	if !foundWidth {
		t.Fatal("missing before/after width")
	}
	var original models.InteriorDoor
	a.db.First(&original, order.InteriorDoors[0].ID)
	if original.Width != 80 {
		t.Fatal("preview changed the actual order")
	}

	apply := func(actor int, row models.OrderDraft) *httptest.ResponseRecorder {
		return a.request(actor, "POST", "/order-drafts/"+row.ID+"/apply", map[string]any{"baseRevision": row.Revision})
	}
	applied := readRow(t, apply(0, first))
	if applied.State != "converted" || applied.OrderID == nil || *applied.OrderID != order.ID {
		t.Fatal("draft was not marked applied")
	}
	readRow(t, apply(0, first)) // Lost response retry must not add another product or bump the revision.
	if legacy := a.request(1, "PUT", path, orderPayload("", 0)); legacy.Code != 409 {
		t.Fatalf("legacy client overwrote a reviewed edit: %d", legacy.Code)
	}
	if stale := apply(1, second); stale.Code != 409 {
		t.Fatalf("stale proposal applied: %d %s", stale.Code, stale.Body.String())
	}
	preserved := readRow(t, a.request(1, "GET", "/order-drafts/"+second.ID, nil))
	if preserved.State != "active" || len(preserved.Document.Changes) == 0 {
		t.Fatal("conflict erased the working copy")
	}

	var updated models.Order
	if err := a.db.Preload("InteriorDoors").Preload("Panelings").Preload("Payments").First(&updated, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if len(updated.InteriorDoors) != 2 || len(updated.Payments) != 1 || updated.Prepayment != 500 ||
		updated.Discount != 300 || updated.Status != "ordered" || updated.Revision != order.Revision+1 ||
		updated.Price != 2926 || len(updated.Panelings[0].Sizes) != 2 {
		t.Fatalf("incorrect order after apply: %+v", updated)
	}
	for _, door := range updated.InteriorDoors {
		if door.Width != 70 || door.Width2 == nil || *door.Width2 != 40 {
			t.Fatal("door or second leaf lost")
		}
		if door.ID == original.ID && (door.CostPrice != 700 || door.Supplier != "Test supplier") {
			t.Fatal("protected fields changed")
		}
	}
}
