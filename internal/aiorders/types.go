package aiorders

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

//go:embed schema.json
var SchemaJSON []byte

//go:embed prompt.txt
var Prompt string

type Field struct {
	ValueLabels map[string]string `json:"valueLabels,omitempty"`
	Max         float64           `json:"max,omitempty"`
	Integer     bool              `json:"integer,omitempty"`
	Label       string            `json:"label"`
	Type        string            `json:"type"`
	Required    bool              `json:"required"`
	Options     []string          `json:"options,omitempty"`
	Min         float64           `json:"min,omitempty"`
	Unit        string            `json:"unit,omitempty"`
}
type ItemSchema struct {
	Label      string           `json:"label"`
	Collection string           `json:"collection"`
	Fields     map[string]Field `json:"fields"`
}
type Schema struct {
	Header map[string]Field      `json:"header"`
	Items  map[string]ItemSchema `json:"items"`
}

var Definitions = func() Schema {
	var s Schema
	if err := json.Unmarshal(SchemaJSON, &s); err != nil {
		panic(err)
	}
	return s
}()

type Item struct {
	ItemID string         `json:"itemId"`
	Type   string         `json:"type"`
	Fields map[string]any `json:"fields"`
}
type Order struct {
	Header map[string]any `json:"header"`
	Items  []Item         `json:"items"`
	Notes  []string       `json:"notes"`
}
type Source struct {
	Text   string `json:"text"`
	Manual bool   `json:"manual"`
}
type Message struct {
	Role string `json:"role"`
	Text string `json:"text"`
}
type Request struct {
	RequestID     string            `json:"requestId"`
	DraftID       string            `json:"draftId"`
	BaseRevision  int               `json:"baseRevision"`
	SchemaVersion int               `json:"schemaVersion"`
	Mode          string            `json:"mode"`
	Text          string            `json:"text"`
	CurrentDraft  Order             `json:"currentDraft"`
	FieldSources  map[string]Source `json:"fieldSources"`
	History       []Message         `json:"history"`
}
type Result struct {
	Order     Order    `json:"order"`
	Questions []string `json:"questions"`
}
type Change struct {
	Path   string `json:"path"`
	Label  string `json:"label"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}
type Issue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}
type Response struct {
	RequestID     string   `json:"requestId"`
	DraftID       string   `json:"draftId"`
	BaseRevision  int      `json:"baseRevision"`
	SchemaVersion int      `json:"schemaVersion"`
	ProposedDraft Order    `json:"proposedDraft"`
	Questions     []string `json:"questions"`
	Issues        []Issue  `json:"issues"`
	Changes       []Change `json:"changes"`
}

var identifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

func DecodeStrict(data []byte, value any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("expected one JSON object")
	}
	return nil
}
func ValidateRequest(r Request) error {
	if !identifier.MatchString(r.RequestID) || !identifier.MatchString(r.DraftID) || r.BaseRevision < 0 || r.SchemaVersion != 1 {
		return errors.New("invalid request metadata")
	}
	if r.Mode != "initial" && r.Mode != "refine" && r.Mode != "edit_order" {
		return errors.New("invalid mode")
	}
	if strings.TrimSpace(r.Text) == "" || len([]rune(r.Text)) > 10000 || len(r.History) > 40 || len(r.FieldSources) > 5000 {
		return errors.New("request limits exceeded")
	}
	for _, m := range r.History {
		if (m.Role != "user" && m.Role != "assistant") || len([]rune(m.Text)) > 10000 {
			return errors.New("invalid history")
		}
	}
	for p, s := range r.FieldSources {
		if len(p) > 200 || len([]rune(s.Text)) > 10000 {
			return errors.New("invalid source")
		}
	}
	return ValidateOrder(r.CurrentDraft)
}
func ValidateOrder(o Order) error {
	if o.Header == nil || o.Items == nil || o.Notes == nil || len(o.Items) > 100 || len(o.Notes) > 100 {
		return errors.New("invalid order shape")
	}
	if err := validateFields(o.Header, Definitions.Header); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, item := range o.Items {
		def, ok := Definitions.Items[item.Type]
		if !ok || !identifier.MatchString(item.ItemID) || seen[item.ItemID] || item.Fields == nil {
			return errors.New("invalid item")
		}
		seen[item.ItemID] = true
		if err := validateFields(item.Fields, def.Fields); err != nil {
			return err
		}
	}
	for _, n := range o.Notes {
		if len([]rune(n)) > 2000 {
			return errors.New("note too long")
		}
	}
	return nil
}
func validateFields(values map[string]any, fields map[string]Field) error {
	for k, v := range values {
		f, ok := fields[k]
		if !ok {
			return fmt.Errorf("unknown field %s", k)
		}
		if v == nil {
			continue
		}
		switch f.Type {
		case "string":
			s, ok := v.(string)
			if !ok || len([]rune(s)) > 2000 {
				return fmt.Errorf("invalid string %s", k)
			}
			if len(f.Options) > 0 && !contains(f.Options, s) {
				return fmt.Errorf("invalid option %s", k)
			}
		case "number":
			n, ok := v.(float64)
			if !ok || math.IsNaN(n) || math.IsInf(n, 0) || n < f.Min || n > 1e9 || (f.Max > 0 && n > f.Max) || (f.Integer && math.Trunc(n) != n) {
				return fmt.Errorf("invalid number %s", k)
			}
		case "boolean":
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("invalid boolean %s", k)
			}
		default:
			return errors.New("invalid schema")
		}
	}
	return nil
}
func contains(a []string, v string) bool {
	for _, s := range a {
		if s == v {
			return true
		}
	}
	return false
}
func flatten(o Order) map[string]any {
	out := map[string]any{}
	for k, v := range o.Header {
		if v != nil && v != "" {
			out["header."+k] = v
		}
	}
	for _, i := range o.Items {
		out["items."+i.ItemID+".type"] = i.Type
		for k, v := range i.Fields {
			if v != nil && v != "" {
				out["items."+i.ItemID+"."+k] = v
			}
		}
	}
	for i, n := range o.Notes {
		out[fmt.Sprintf("notes.%d", i)] = n
	}
	return out
}
func Changes(before, after Order) []Change {
	a, b := flatten(before), flatten(after)
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	sorted := []string{}
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	out := []Change{}
	for _, p := range sorted {
		if !reflect.DeepEqual(a[p], b[p]) {
			label := p
			parts := strings.Split(p, ".")
			if parts[0] == "header" {
				label = Definitions.Header[parts[1]].Label
			} else if parts[0] == "items" {
				for _, item := range append(append([]Item{}, after.Items...), before.Items...) {
					if item.ItemID == parts[1] {
						if parts[2] == "type" {
							label = Definitions.Items[item.Type].Label
						} else {
							label = Definitions.Items[item.Type].Label + ": " + Definitions.Items[item.Type].Fields[parts[2]].Label
						}
						break
					}
				}
			}
			out = append(out, Change{p, label, a[p], b[p]})
		}
	}
	return out
}
func ValidateResult(r Request, result Result) error {
	if err := ValidateOrder(result.Order); err != nil {
		return err
	}
	if result.Questions == nil || len(result.Questions) > 20 {
		return errors.New("invalid questions")
	}
	for _, q := range result.Questions {
		if len([]rune(q)) > 1000 {
			return errors.New("question too long")
		}
	}
	before, after := flatten(r.CurrentDraft), flatten(result.Order)
	for p, s := range r.FieldSources {
		if s.Manual && !reflect.DeepEqual(before[p], after[p]) {
			return errors.New("manual field changed")
		}
	}
	// Existing item IDs must not change type, even if both types have compatible fields.
	for _, old := range r.CurrentDraft.Items {
		for _, next := range result.Order.Items {
			if old.ItemID == next.ItemID && old.Type != next.Type {
				return errors.New("item type changed")
			}
		}
	}
	return nil
}
func Issues(o Order) []Issue {
	out := []Issue{}
	check := func(prefix string, values map[string]any, defs map[string]Field) {
		keys := []string{}
		for k := range defs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			f := defs[k]
			if f.Required && (values[k] == nil || values[k] == "") {
				out = append(out, Issue{prefix + k, "Не указано: " + f.Label})
			}
		}
	}
	check("header.", o.Header, Definitions.Header)
	if s, ok := o.Header["phone"].(string); ok && s != "" && !regexp.MustCompile(`^7\d{10}$`).MatchString(s) {
		out = append(out, Issue{"header.phone", "Проверьте телефон: нужно 11 цифр с начальной 7"})
	}
	if s, ok := o.Header["date"].(string); ok && s != "" {
		if _, err := time.Parse("2006-01-02", s); err != nil {
			out = append(out, Issue{"header.date", "Проверьте дату заказа"})
		}
	}
	if o.Header["needsDelivery"] == true && (o.Header["deliveryAddress"] == nil || o.Header["deliveryAddress"] == "") {
		out = append(out, Issue{"header.deliveryAddress", "Укажите адрес доставки"})
	}
	for _, i := range o.Items {
		p := "items." + i.ItemID + "."
		check(p, i.Fields, Definitions.Items[i.Type].Fields)
		if i.Type == "interiorDoor" && i.Fields["hasGlass"] == true && (i.Fields["glassComment"] == nil || i.Fields["glassComment"] == "") {
			out = append(out, Issue{p + "glassComment", "Опишите стекло"})
		}
		if i.Type == "interiorDoor" && i.Fields["leafType"] == "Double" {
			for _, k := range []string{"width2", "height2", "count2", "price2"} {
				if i.Fields[k] == nil {
					out = append(out, Issue{p + k, "Уточните второе полотно: " + Definitions.Items[i.Type].Fields[k].Label})
				}
			}
		}
		if i.Type == "hardware" {
			found := false
			for k, v := range i.Fields {
				if strings.HasSuffix(k, "Count") && v != nil {
					found = true
					price := strings.TrimSuffix(k, "Count") + "Price"
					if _, exists := Definitions.Items[i.Type].Fields[price]; exists && i.Fields[price] == nil {
						out = append(out, Issue{p + price, "Укажите цену: " + Definitions.Items[i.Type].Fields[price].Label})
					}
				}
			}
			if !found {
				out = append(out, Issue{p + "handleCount", "Укажите состав фурнитуры"})
			}
		}
	}
	return out
}
