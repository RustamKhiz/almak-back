package draftorders

import (
	"almak-back/internal/aiorders"
	"encoding/json"
	"errors"
	"regexp"
)

var Identifier = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type EditorState struct {
	Payload           json.RawMessage `json:"payload"`
	Accepted          bool            `json:"accepted"`
	DefaultsConfirmed bool            `json:"defaultsConfirmed"`
}
type Previous struct {
	Order        aiorders.Order             `json:"order"`
	FieldSources map[string]aiorders.Source `json:"fieldSources"`
}
type Document struct {
	SourceText        string                     `json:"sourceText"`
	PendingText       string                     `json:"pendingText"`
	Order             *aiorders.Order            `json:"order,omitempty"`
	FieldSources      map[string]aiorders.Source `json:"fieldSources"`
	Questions         []string                   `json:"questions"`
	Changes           []aiorders.Change          `json:"changes"`
	LastAppliedChange *Previous                  `json:"lastAppliedChange,omitempty"`
	EditorState       *EditorState               `json:"editorState,omitempty"`
	Payload           json.RawMessage            `json:"payload,omitempty"`
}

func Validate(kind string, doc Document) error {
	if len([]rune(doc.SourceText)) > 10000 || len([]rune(doc.PendingText)) > 10000 || len(doc.FieldSources) > 5000 || len(doc.Questions) > 100 || len(doc.Changes) > 5000 {
		return errors.New("draft limits exceeded")
	}
	for path, source := range doc.FieldSources {
		if len(path) > 200 || len([]rune(source.Text)) > 10000 {
			return errors.New("invalid source")
		}
	}
	for _, q := range doc.Questions {
		if len([]rune(q)) > 2000 {
			return errors.New("invalid question")
		}
	}
	if kind == "ai" {
		if doc.Order == nil {
			return errors.New("missing partial order")
		}
		if err := aiorders.ValidateOrder(*doc.Order); err != nil {
			return err
		}
		if doc.LastAppliedChange != nil {
			if err := aiorders.ValidateOrder(doc.LastAppliedChange.Order); err != nil {
				return err
			}
		}
		if doc.EditorState != nil && len(doc.EditorState.Payload) > 0 && string(doc.EditorState.Payload) != "null" {
			if err := ValidatePayload(doc.EditorState.Payload); err != nil {
				return err
			}
		}
	} else if kind == "manual" {
		return ValidatePayload(doc.Payload)
	} else {
		return errors.New("invalid draft kind")
	}
	return nil
}

func ValidatePayload(raw json.RawMessage) error {
	var p map[string]json.RawMessage
	if json.Unmarshal(raw, &p) != nil || p == nil {
		return errors.New("invalid editor payload")
	}
	collections := map[string]string{}
	for kind, definition := range aiorders.Definitions.Items {
		collections[definition.Collection] = kind
	}
	for collection := range collections {
		if value, ok := p[collection]; !ok || string(value) == "null" {
			return errors.New("missing product collection")
		}
	}
	total := 0
	for key, value := range p {
		if kind, ok := collections[key]; ok {
			var items []map[string]json.RawMessage
			if json.Unmarshal(value, &items) != nil {
				return errors.New("invalid product array")
			}
			total += len(items)
			if total > 100 {
				return errors.New("too many products")
			}
			for _, item := range items {
				if item == nil {
					return errors.New("invalid product")
				}
				fields := map[string]any{}
				for field, rawValue := range item {
					if _, known := aiorders.Definitions.Items[kind].Fields[field]; known {
						var v any
						if json.Unmarshal(rawValue, &v) != nil {
							return errors.New("invalid field")
						}
						fields[field] = v
					} else {
						switch field {
						case "sizes":
							var sizes []map[string]float64
							if string(rawValue) == "null" || json.Unmarshal(rawValue, &sizes) != nil || len(sizes) > 100 {
								return errors.New("invalid product sizes")
							}
							for _, size := range sizes {
								if size == nil {
									return errors.New("invalid product size")
								}
								for key, n := range size {
									if (key != "width" && key != "height" && key != "quantity") || n < 0 {
										return errors.New("invalid product size")
									}
								}
							}
						case "id", "totalArea", "setCount":
							var n float64
							if string(rawValue) == "null" || json.Unmarshal(rawValue, &n) != nil || n < 0 {
								return errors.New("invalid product number")
							}
						case "type", "size":
							var s string
							if string(rawValue) == "null" || json.Unmarshal(rawValue, &s) != nil || len(s) > 1000 {
								return errors.New("invalid product text")
							}
						default:
							return errors.New("unknown product field")
						}
					}
				}
				order := aiorders.Order{Header: map[string]any{}, Items: []aiorders.Item{{ItemID: "validate", Type: kind, Fields: fields}}, Notes: []string{}}
				if err := aiorders.ValidateOrder(order); err != nil {
					return err
				}
			}
		} else {
			if string(value) == "null" {
				return errors.New("invalid header value")
			}
			switch key {
			case "status":
				var n int
				if json.Unmarshal(value, &n) == nil && n >= 1 && n <= 6 {
					break
				}
				var s string
				if json.Unmarshal(value, &s) != nil || len(s) > 100 {
					return errors.New("invalid editor status")
				}
			case "name", "phone", "date", "deliveryAddress", "comment":
				var s string
				if json.Unmarshal(value, &s) != nil || len([]rune(s)) > 10000 {
					return errors.New("invalid header text")
				}
			case "prepayment", "discount":
				var n float64
				if json.Unmarshal(value, &n) != nil || n < 0 {
					return errors.New("invalid amount")
				}
			case "needsDelivery", "isPaid":
				var b bool
				if json.Unmarshal(value, &b) != nil {
					return errors.New("invalid flag")
				}
			case "payments":
				var payments []json.RawMessage
				if json.Unmarshal(value, &payments) != nil || len(payments) > 1000 {
					return errors.New("invalid payments")
				}
			default:
				return errors.New("unknown editor field")
			}
		}
	}
	return nil
}
