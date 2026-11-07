package handlers

import (
	"strings"
	"testing"

	"almak-back/internal/aiorders"
)

func TestDraftReplyReportsMissingFieldsWithoutQuestions(t *testing.T) {
	order := aiorders.Order{
		Header: map[string]any{"name": "Тест", "phone": "79000000000", "date": "2026-11-07", "needsDelivery": false},
		Items: []aiorders.Item{{
			ItemID: "door-1", Type: "interiorDoor",
			Fields: map[string]any{"model": "Верона", "count": float64(2), "width": float64(80), "height": float64(200),
				"price": float64(0), "hasGlass": false, "leafType": "Single"},
		}},
		Notes: []string{"Цена комплекта не определена."},
	}
	reply := draftReply("initial", order, 1)
	for _, expected := range []string{"Черновик создан.", "Позиция 1", "Цвет", "Покрытие", "Цена комплекта не определена."} {
		if !strings.Contains(reply, expected) {
			t.Fatalf("missing %q in %s", expected, reply)
		}
	}
	for _, absent := range []string{"?", "Цена, ₽", "Телефон", "Есть стекло", "Ширина, см"} {
		if strings.Contains(reply, absent) {
			t.Fatalf("unexpected %q in %s", absent, reply)
		}
	}
	order.Items[0].Fields["color"] = "Белый"
	order.Items[0].Fields["covering"] = "Эмаль"
	reply = draftReply("refine", order, 1)
	if !strings.Contains(reply, "Черновик обновлён.") || !strings.Contains(reply, "Обязательные поля заполнены.") {
		t.Fatal(reply)
	}
	delete(order.Items[0].Fields, "leafType")
	order.Notes = []string{"Полотно: Single/Double."}
	reply = draftReply("refine", order, 1)
	if strings.Contains(reply, "Single") || strings.Contains(reply, "Double") ||
		!strings.Contains(reply, "Одностворчатая / Двустворчатая") {
		t.Fatal(reply)
	}
}

func TestEditReplyKeepsReviewStepAndOmitsUneditableDate(t *testing.T) {
	order := aiorders.Order{
		Header: map[string]any{"name": "Тест", "phone": "79000000000", "needsDelivery": false},
		Items:  []aiorders.Item{}, Notes: []string{"Не удалось определить позицию для изменения."},
	}
	reply := draftReply("edit_order", order, 1)
	if !strings.Contains(reply, "перед применением") || !strings.Contains(reply, order.Notes[0]) || strings.Contains(reply, "Дата заказа") {
		t.Fatal(reply)
	}
}
