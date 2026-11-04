package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"almak-back/internal/config"
	"almak-back/internal/models"
	"almak-back/internal/routes"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const draftOne = "11111111-1111-4111-8111-111111111111"
const draftTwo = "22222222-2222-4222-8222-222222222222"
const testJWT = "draft-integration-test-key"

type draftAPI struct {
	db       *gorm.DB
	router   http.Handler
	tokens   []string
	sequence atomic.Uint64
}

func setupDraftAPI(t *testing.T, provider string) *draftAPI {
	t.Helper()
	db := isolatedDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.Order{}, &models.OrderPayment{}, &models.InteriorDoor{}, &models.EntranceDoor{}, &models.Molding{}, &models.Extension{}, &models.Capital{}, &models.Hardware{}, &models.Paneling{}, &models.Skirting{}, &models.OrderDraft{}, &models.DraftMessage{}, &models.DraftAIRequest{}, &models.DraftMutation{}, &models.Catalog{}, &models.CatalogItem{}); err != nil {
		t.Fatal(err)
	}
	api := &draftAPI{db: db}
	for i, role := range []models.UserRole{models.RoleAdmin, models.RoleManager} {
		user := models.User{Login: fmt.Sprint("user", i), Password: "not-used", Role: role}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user.ID, "type": "access", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(testJWT))
		api.tokens = append(api.tokens, token)
	}
	gin.SetMode(gin.TestMode)
	api.router = routes.SetupRouter(config.Config{JWTSecret: testJWT, FrontendOrigins: []string{"http://localhost:4200"}, AIOrderDraftsEnabled: provider != "", DeepSeekAPIKey: "isolated-provider-key", DeepSeekBaseURL: provider, DeepSeekModel: "test"})
	return api
}
func (a *draftAPI) request(actor int, method, path string, body any) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	r := httptest.NewRequest(method, "/api"+path, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	if actor >= 0 {
		r.Header.Set("Authorization", "Bearer "+a.tokens[actor])
	}
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, r)
	return w
}
func (a *draftAPI) writeBody(id, kind string, revision int, doc any) map[string]any {
	return map[string]any{"id": id, "kind": kind, "baseRevision": revision, "requestId": fmt.Sprintf("00000000-0000-4000-8000-%012d", a.sequence.Add(1)), "document": doc}
}
func partialDoc(name string) map[string]any {
	return map[string]any{"sourceText": "test", "pendingText": "test", "order": map[string]any{"header": map[string]any{"name": name}, "items": []any{}, "notes": []any{}}, "fieldSources": map[string]any{}, "questions": []any{}, "changes": []any{}}
}
func readRow(t *testing.T, w *httptest.ResponseRecorder) models.OrderDraft {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("draft response %d: %s", w.Code, w.Body.String())
	}
	var row models.OrderDraft
	if err := json.Unmarshal(w.Body.Bytes(), &row); err != nil {
		t.Fatal(err)
	}
	return row
}

func TestSharedDraftsCASAndArchive(t *testing.T) {
	a := setupDraftAPI(t, "")
	create := a.writeBody(draftOne, "ai", 0, partialDoc("One"))
	first := readRow(t, a.request(0, "POST", "/order-drafts", create))
	if first.CreatedBy == nil || *first.CreatedBy != 1 || first.Revision != 1 {
		t.Fatal("incorrect server authorship")
	}
	if readRow(t, a.request(0, "POST", "/order-drafts", create)).Revision != 1 {
		t.Fatal("retry duplicated create")
	}
	shared := readRow(t, a.request(1, "GET", "/order-drafts/"+draftOne, nil))
	if shared.Document.Order.Header["name"] != "One" {
		t.Fatal("not shared")
	}
	write := a.writeBody(draftOne, "ai", 1, partialDoc("Manager edit"))
	updated := readRow(t, a.request(1, "PUT", "/order-drafts/"+draftOne, write))
	if updated.Revision != 2 || updated.UpdatedBy != 2 {
		t.Fatal("wrong editor")
	}
	if readRow(t, a.request(1, "PUT", "/order-drafts/"+draftOne, write)).Revision != 2 {
		t.Fatal("retry duplicated write")
	}
	if a.request(0, "PUT", "/order-drafts/"+draftOne, a.writeBody(draftOne, "ai", 1, partialDoc("Stale"))).Code != 409 {
		t.Fatal("stale write accepted")
	}
	if a.request(-1, "GET", "/order-drafts", nil).Code != 401 {
		t.Fatal("unauthenticated access")
	}
	forged := a.writeBody(draftTwo, "ai", 0, partialDoc("forged"))
	forged["createdBy"] = 2
	if a.request(0, "POST", "/order-drafts", forged).Code != 400 {
		t.Fatal("client authorship accepted")
	}
	archived := readRow(t, a.request(1, "POST", "/order-drafts/"+draftOne+"/archive", a.writeBody(draftOne, "ai", 2, nil)))
	if archived.State != "archived" {
		t.Fatal("archive failed")
	}
	list := a.request(0, "GET", "/order-drafts", nil)
	var result struct{ Total int }
	json.Unmarshal(list.Body.Bytes(), &result)
	if result.Total != 0 {
		t.Fatal("archive still listed")
	}
	restored := readRow(t, a.request(0, "POST", "/order-drafts/"+draftOne+"/restore", a.writeBody(draftOne, "ai", 3, nil)))
	if restored.State != "active" {
		t.Fatal("restore failed")
	}
	// A retry after another successful mutation must surface a conflict, not rebase stale input.
	if a.request(1, "PUT", "/order-drafts/"+draftOne, write).Code != 409 {
		t.Fatal("old retry overwrote a newer revision")
	}
}

func TestImportPreservesHistoryAndDoesNotOverwriteNewerWork(t *testing.T) {
	a := setupDraftAPI(t, "")
	body := a.writeBody(draftTwo, "ai", 0, partialDoc("Imported"))
	body["sourceKey"] = "ai:" + draftTwo
	body["messages"] = []any{map[string]any{"role": "user", "text": "old note", "createdAt": "2026-10-01T10:00:00Z", "authorId": 1}}
	imported := readRow(t, a.request(1, "POST", "/order-drafts/import", body))
	if imported.CreatedBy != nil || imported.ImportedBy == nil || *imported.ImportedBy != 2 || len(imported.Messages) != 1 || imported.Messages[0].AuthorID != nil {
		t.Fatal("legacy authorship or history lost")
	}
	readRow(t, a.request(0, "PUT", "/order-drafts/"+draftTwo, a.writeBody(draftTwo, "ai", 1, partialDoc("New server edit"))))
	retried := readRow(t, a.request(0, "POST", "/order-drafts/import", body))
	if retried.Document.Order.Header["name"] != "New server edit" || len(retried.Messages) != 1 {
		t.Fatal("import overwrote or duplicated data")
	}
	body["document"] = partialDoc("Different local copy")
	if a.request(0, "POST", "/order-drafts/import", body).Code != 409 {
		t.Fatal("different import overwrote data")
	}
	manual := map[string]any{"name": "Manual", "phone": "", "date": "", "comment": "", "deliveryAddress": "", "status": 1, "needsDelivery": false, "isPaid": false, "prepayment": 0, "discount": 0, "payments": []any{}}
	for _, key := range []string{"interiorDoors", "entranceDoors", "moldings", "extensions", "capitals", "hardwares", "panelings", "skirtings"} {
		manual[key] = []any{}
	}
	doc := map[string]any{"payload": manual, "sourceText": "", "pendingText": "", "fieldSources": map[string]any{}, "questions": []any{}, "changes": []any{}}
	body = a.writeBody(draftOne, "manual", 0, doc)
	body["sourceKey"] = "manual:profile:1"
	body["messages"] = []any{}
	row := readRow(t, a.request(0, "POST", "/order-drafts/import", body))
	if row.Kind != "manual" || len(row.Document.Payload) == 0 {
		t.Fatal("manual draft lost")
	}
}

func orderPayload(id string, revision int) map[string]any {
	return map[string]any{"draftId": id, "draftRevision": revision, "customer": "Customer", "phone": "79000000000", "date": "2026-11-03", "status": "accepted", "prepayment": 10000, "discount": 0, "interiorDoors": []any{map[string]any{"model": "Verona", "color": "White", "price": 12000, "count": 2, "width": 80, "height": 200, "leafType": "Single", "covering": "Эмаль"}}}
}

func TestRepeatOrderCopiesProductsIntoIndependentDraft(t *testing.T) {
	a := setupDraftAPI(t, "")
	order := models.Order{
		Customer: "Repeat customer", Phone: "79000000000", Date: "2026-11-01",
		NeedsDelivery: true, DeliveryAddress: "Test street", Status: "completed",
		Comment: "Original comment", Discount: 300, Prepayment: 800, IsPaid: true,
		Payments:      []models.OrderPayment{{Amount: 800}},
		InteriorDoors: []models.InteriorDoor{{ID: 42, Model: "Verona", CostPrice: 500, Price: 1000, Width: 80, Height: 200, Count: 1}},
		EntranceDoors: []models.EntranceDoor{{Model: "Entry", Width: 90, Height: 200, Count: 1}},
		Moldings:      []models.Molding{{FramePrice: 100, FrameCount: 2.5}},
		Extensions:    []models.Extension{{Width: 20, Height: 200, Count: 1, QuantityPerSet: 0.5}},
		Capitals:      []models.Capital{{Name: "Capital", Width: 80, Height: 20, Count: 1}},
		Hardwares:     []models.Hardware{{Supplier: "Supplier"}},
		Panelings:     []models.Paneling{{Size: "20x30; 40х50", Width: 20, Height: 30, Count: 2}},
		Skirtings:     []models.Skirting{{Model: "Skirting", Height: 10, Length: 2.5, Count: 1}},
	}
	if err := a.db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/orders/%d/repeat", order.ID)
	options := map[string]any{"requestId": draftOne}
	row := readRow(t, a.request(1, "POST", path, options))
	if row.SourceOrderID == nil || *row.SourceOrderID != order.ID || row.State != "active" || row.OrderID != nil {
		t.Fatalf("incorrect repeat metadata: %+v", row)
	}
	var payload map[string]any
	if err := json.Unmarshal(row.Document.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	for _, collection := range []string{"interiorDoors", "entranceDoors", "moldings", "extensions", "capitals", "hardwares", "panelings", "skirtings"} {
		items := payload[collection].([]any)
		if len(items) != 1 {
			t.Fatalf("missing product in %s", collection)
		}
		item := items[0].(map[string]any)
		if item["id"] != float64(1) || item["order_id"] != nil || item["costPrice"] != float64(0) {
			t.Fatalf("source data leaked into product %s", collection)
		}
	}
	if payload["date"] != "" || payload["comment"] != "" || payload["discount"] != float64(0) ||
		payload["prepayment"] != float64(0) || payload["isPaid"] != false || len(payload["payments"].([]any)) != 0 {
		t.Fatal("repeat retained source date, comment, discount or payment state")
	}
	if got := len(payload["panelings"].([]any)[0].(map[string]any)["sizes"].([]any)); got != 2 {
		t.Fatalf("legacy paneling dimensions lost: %d", got)
	}
	if retry := readRow(t, a.request(1, "POST", path, options)); retry.ID != row.ID {
		t.Fatal("retry created another draft")
	}
	options = map[string]any{"requestId": draftTwo, "copyComment": true, "copyDiscount": true}
	withOptions := readRow(t, a.request(1, "POST", path, options))
	if err := json.Unmarshal(withOptions.Document.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["comment"] != order.Comment || payload["discount"] != order.Discount || withOptions.ID == row.ID {
		t.Fatal("explicit repeat options were not applied")
	}
	var orders, payments int64
	a.db.Model(&models.Order{}).Count(&orders)
	a.db.Model(&models.OrderPayment{}).Count(&payments)
	if orders != 1 || payments != 1 {
		t.Fatalf("repeat changed registered orders or payments: %d, %d", orders, payments)
	}
}

func TestOrderRevisionProtectsConcurrentEditsAndPayments(t *testing.T) {
	a := setupDraftAPI(t, "")
	original := orderPayload("", 0)
	created := a.request(0, "POST", "/orders", original)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var order models.Order
	if err := json.Unmarshal(created.Body.Bytes(), &order); err != nil || order.Revision == 0 {
		t.Fatalf("missing initial revision: %v", err)
	}
	path := fmt.Sprintf("/orders/%d", order.ID)
	oldRevision := order.Revision
	if payment := a.request(1, "POST", path+"/payments", map[string]any{"amount": 100}); payment.Code != http.StatusOK {
		t.Fatalf("payment: %d %s", payment.Code, payment.Body.String())
	}
	original["baseRevision"] = oldRevision
	if stale := a.request(0, "PUT", path, original); stale.Code != http.StatusConflict {
		t.Fatalf("stale update after payment: %d %s", stale.Code, stale.Body.String())
	}
	current := a.request(0, "GET", path, nil)
	if err := json.Unmarshal(current.Body.Bytes(), &order); current.Code != http.StatusOK || err != nil {
		t.Fatalf("read current: %d %v", current.Code, err)
	}
	if order.Revision != oldRevision+1 || order.Prepayment != 10100 || len(order.Payments) != 2 {
		t.Fatal("stale edit changed payments or revision")
	}
	baseRevision := order.Revision
	original["baseRevision"] = baseRevision
	original["prepayment"] = order.Prepayment
	var group sync.WaitGroup
	codes := make(chan int, 2)
	for actor := range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			codes <- a.request(actor, "PUT", path, original).Code
		}()
	}
	group.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[http.StatusOK] != 1 || counts[http.StatusConflict] != 1 {
		t.Fatalf("concurrent edits returned %v", counts)
	}
}
func TestDraftConversionIsAtomicAndIdempotent(t *testing.T) {
	a := setupDraftAPI(t, "")
	readRow(t, a.request(0, "POST", "/order-drafts", a.writeBody(draftOne, "ai", 0, partialDoc("Draft"))))
	a.db.Callback().Create().Before("gorm:create").Register("reject-test-payment", func(tx *gorm.DB) {
		if tx.Statement.Table == "order_payments" {
			tx.AddError(fmt.Errorf("test payment failure"))
		}
	})
	if a.request(0, "POST", "/orders", orderPayload(draftOne, 1)).Code != 500 {
		t.Fatal("payment failure not returned")
	}
	a.db.Callback().Create().Remove("reject-test-payment")
	var count int64
	a.db.Model(&models.Order{}).Count(&count)
	if count != 0 {
		t.Fatal("failed conversion created an order")
	}
	unchanged := readRow(t, a.request(1, "GET", "/order-drafts/"+draftOne, nil))
	if unchanged.State != "active" || unchanged.Revision != 1 {
		t.Fatal("failed conversion closed draft")
	}
	var group sync.WaitGroup
	responses := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		group.Add(1)
		go func() { defer group.Done(); responses <- a.request(0, "POST", "/orders", orderPayload(draftOne, 1)) }()
	}
	group.Wait()
	close(responses)
	var orderID uint
	for response := range responses {
		if response.Code != 201 {
			t.Fatalf("conversion %d: %s", response.Code, response.Body.String())
		}
		var order models.Order
		json.Unmarshal(response.Body.Bytes(), &order)
		if orderID != 0 && orderID != order.ID {
			t.Fatal("different orders created")
		}
		orderID = order.ID
	}
	a.db.Model(&models.Order{}).Count(&count)
	if count != 1 {
		t.Fatal("order duplicated")
	}
	a.db.Model(&models.OrderPayment{}).Count(&count)
	if count != 1 {
		t.Fatal("initial payment duplicated")
	}
	converted := readRow(t, a.request(1, "GET", "/order-drafts/"+draftOne, nil))
	if converted.OrderID == nil || *converted.OrderID != orderID || converted.State != "converted" {
		t.Fatal("draft link missing")
	}
}

func TestServerChatPersistsInputAndRejectsLateAIResult(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		var body struct{ Messages []struct{ Content string } }
		json.NewDecoder(r.Body).Decode(&body)
		var input struct {
			Request struct {
				CurrentDraft struct{ Header map[string]any }
			}
		}
		json.Unmarshal([]byte(body.Messages[1].Content), &input)
		if n == 1 {
			close(started)
			<-release
		}
		name := "Generated"
		if n > 1 {
			name = "Manual"
		}
		result := map[string]any{"order": map[string]any{"header": map[string]any{"name": name}, "items": []any{}, "notes": []any{}}, "questions": []any{}}
		content, _ := json.Marshal(result)
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(content)}, "finish_reason": "stop"}}})
	}))
	defer provider.Close()
	a := setupDraftAPI(t, provider.URL)
	doc := partialDoc("Before")
	readRow(t, a.request(0, "POST", "/order-drafts", a.writeBody(draftOne, "ai", 0, doc)))
	command := map[string]any{"requestId": "33333333-3333-4333-8333-333333333333", "baseRevision": 1, "text": "test"}
	done := make(chan *httptest.ResponseRecorder)
	go func() { done <- a.request(0, "POST", "/order-drafts/"+draftOne+"/messages", command) }()
	<-started
	during := readRow(t, a.request(1, "GET", "/order-drafts/"+draftOne, nil))
	if len(during.Messages) != 1 || during.Messages[0].Role != "user" {
		t.Fatal("input not persisted before provider call")
	}
	readRow(t, a.request(1, "PUT", "/order-drafts/"+draftOne, a.writeBody(draftOne, "ai", 1, partialDoc("Manual"))))
	close(release)
	if (<-done).Code != 409 {
		t.Fatal("late result applied")
	}
	after := readRow(t, a.request(1, "GET", "/order-drafts/"+draftOne, nil))
	if after.Document.Order.Header["name"] != "Manual" || len(after.Messages) != 1 {
		t.Fatal("manual work or input lost")
	}
	command["baseRevision"] = 2
	command["requestId"] = "44444444-4444-4444-8444-444444444444"
	success := readRow(t, a.request(0, "POST", "/order-drafts/"+draftOne+"/messages", command))
	if len(success.Messages) != 2 || success.Document.PendingText != "" {
		t.Fatal("retry duplicated input or did not finish")
	}
	readRow(t, a.request(0, "POST", "/order-drafts/"+draftOne+"/messages", command))
	if calls.Load() != 2 {
		t.Fatal("completed request called provider again")
	}
}
