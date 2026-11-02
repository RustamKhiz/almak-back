package aiorders

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func emptyOrder() Order { return Order{Header: map[string]any{}, Items: []Item{}, Notes: []string{}} }
func TestValidatePartialAndRejectUnknownFields(t *testing.T) {
	o := emptyOrder()
	o.Items = append(o.Items, Item{ItemID: "door-1", Type: "interiorDoor", Fields: map[string]any{"model": "Верона"}})
	if err := ValidateOrder(o); err != nil {
		t.Fatal(err)
	}
	if len(Issues(o)) == 0 {
		t.Fatal("partial order must report missing values")
	}
	o.Items[0].Fields["paymentID"] = "unexpected"
	if ValidateOrder(o) == nil {
		t.Fatal("unknown field accepted")
	}
}
func TestManualFieldsCannotBeChangedOrDeleted(t *testing.T) {
	old := emptyOrder()
	old.Items = []Item{{"one", "interiorDoor", map[string]any{"price": float64(12000)}}}
	req := Request{CurrentDraft: old, FieldSources: map[string]Source{"items.one.price": {Manual: true}}}
	for _, next := range []Order{{Header: map[string]any{}, Items: []Item{{"one", "interiorDoor", map[string]any{"price": float64(13000)}}}, Notes: []string{}}, emptyOrder()} {
		if ValidateResult(req, Result{next, []string{}}) == nil {
			t.Fatal("manual change accepted")
		}
	}
	if err := ValidateResult(req, Result{old, []string{}}); err != nil {
		t.Fatal(err)
	}
}
func TestZeroFalseAndUnknownRemainDistinct(t *testing.T) {
	old := emptyOrder()
	next := emptyOrder()
	next.Header["discount"] = float64(0)
	next.Header["needsDelivery"] = false
	changes := Changes(old, next)
	if len(changes) != 2 {
		t.Fatalf("expected both explicit values, got %#v", changes)
	}
	if err := ValidateOrder(next); err != nil {
		t.Fatal(err)
	}
	next.Header["discount"] = float64(-1)
	if ValidateOrder(next) == nil {
		t.Fatal("negative money accepted")
	}
}
func TestStrictDecodeRejectsTrailingAndUnknownTopLevel(t *testing.T) {
	for _, input := range []string{`{"order":{"header":{},"items":[],"notes":[]},"questions":[],"run":"delete"}`, `{"order":{"header":{},"items":[],"notes":[]},"questions":[]} {}`} {
		var result Result
		if DecodeStrict([]byte(input), &result) == nil {
			t.Fatal("invalid JSON shape accepted")
		}
	}
}
func TestEveryTypeAndSchemaParity(t *testing.T) {
	if len(Definitions.Items) != 8 {
		t.Fatal("missing product types")
	}
	// Check the frontend copy as a JSON object, independently of formatting.
	var value any
	if err := json.Unmarshal(SchemaJSON, &value); err != nil {
		t.Fatal(err)
	}
	copy, err := os.ReadFile("../../../almak-front/src/app/ai-orders/schema.json")
	if err == nil {
		var frontend any
		if json.Unmarshal(copy, &frontend) != nil || !reflect.DeepEqual(value, frontend) {
			t.Fatal("frontend and backend schema differ")
		}
	}
	for kind := range Definitions.Items {
		order := emptyOrder()
		order.Items = []Item{{"one", kind, map[string]any{}}}
		if err := ValidateOrder(order); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
}
