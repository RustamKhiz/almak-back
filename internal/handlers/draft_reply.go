package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"almak-back/internal/aiorders"
)

func draftReply(mode string, before, order aiorders.Order, changes int) string {
	text := "Черновик создан."
	if mode == "refine" {
		text = "Черновик обновлён."
	}
	if mode == "edit_order" {
		text = "Изменения подготовлены. Проверьте «было → стало» перед применением."
		if changes == 0 {
			text = "В рабочей копии пока нет изменений."
		}
	}
	if summary := draftSummary(before, order); len(summary) > 0 {
		text += "\n" + strings.Join(summary, "\n")
	} else if mode != "initial" {
		text += "\nВ этом сообщении новых изменений нет."
	}

	type group struct {
		title  string
		fields []string
	}
	groups := []group{{title: "Заказ"}}
	for i, item := range order.Items {
		groups = append(groups, group{title: fmt.Sprintf("Позиция %d (%s)", i+1, aiorders.Definitions.Items[item.Type].Label)})
	}
	positions := map[string]int{}
	for i, item := range order.Items {
		positions[item.ItemID] = i + 1
	}
	for _, issue := range aiorders.Issues(order) {
		parts := strings.Split(issue.Path, ".")
		if len(parts) < 2 {
			continue
		}
		index := 0
		field := aiorders.Definitions.Header[parts[1]]
		value := order.Header[parts[1]]
		if parts[0] == "items" && len(parts) == 3 {
			index = positions[parts[1]]
			item := order.Items[index-1]
			field = aiorders.Definitions.Items[item.Type].Fields[parts[2]]
			if parts[2] == "type" {
				field = aiorders.Field{Label: "Тип товара", ValueLabels: map[string]string{item.Type: aiorders.Definitions.Items[item.Type].Label}}
			}
			value = item.Fields[parts[2]]
		}
		if mode == "edit_order" && index == 0 && (parts[1] == "date") {
			continue
		}
		label := field.Label
		if label == "" {
			continue
		}
		if value != nil && value != "" {
			label += " (проверьте значение)"
		} else if len(field.Options) > 0 {
			options := []string{}
			for _, option := range field.Options {
				if name := field.ValueLabels[option]; name != "" {
					options = append(options, name)
				}
			}
			if len(options) > 0 {
				label += " (" + strings.Join(options, " / ") + ")"
			}
		}
		groups[index].fields = append(groups[index].fields, label)
	}
	if len(order.Items) == 0 {
		groups[0].fields = append(groups[0].fields, "Товарные позиции")
	}
	lines := []string{}
	for _, group := range groups {
		if len(group.fields) > 0 {
			lines = append(lines, group.title+": "+strings.Join(group.fields, ", "))
		}
	}
	if len(lines) > 0 {
		text += "\n\nОсталось заполнить или проверить:\n• " + strings.Join(lines, "\n• ")
	} else if mode != "edit_order" {
		text += "\nОбязательные поля заполнены."
	}
	if len(order.Notes) > 0 {
		notes := make([]string, len(order.Notes))
		for i, note := range order.Notes {
			notes[i] = aiorders.DisplayText(note)
		}
		text += "\n\nДля проверки:\n• " + strings.Join(notes, "\n• ")
	}
	return text
}

func draftSummary(before, after aiorders.Order) []string {
	previous, current := map[string]aiorders.Item{}, map[string]aiorders.Item{}
	for _, item := range before.Items {
		previous[item.ItemID] = item
	}
	lines := []string{}
	for _, item := range after.Items {
		current[item.ItemID] = item
		if _, exists := previous[item.ItemID]; !exists {
			name := aiorders.Definitions.Items[item.Type].Label
			if model, ok := item.Fields["model"].(string); ok && model != "" {
				name += " · " + model
			}
			if count, ok := item.Fields["count"].(float64); ok {
				name += fmt.Sprintf(" — %g шт.", count)
			}
			lines = append(lines, "Добавлено: "+name)
		}
	}
	for i, item := range before.Items {
		if _, exists := current[item.ItemID]; !exists {
			lines = append(lines, fmt.Sprintf("Удалена позиция %d: %s", i+1, aiorders.Definitions.Items[item.Type].Label))
		}
	}
	for _, change := range aiorders.Changes(before, after) {
		parts := strings.Split(change.Path, ".")
		field := aiorders.Field{}
		label := change.Label
		if parts[0] == "header" {
			field = aiorders.Definitions.Header[parts[1]]
		} else if parts[0] == "items" && len(parts) == 3 {
			item, exists := current[parts[1]]
			if _, existed := previous[parts[1]]; !exists || !existed {
				continue
			}
			field = aiorders.Definitions.Items[item.Type].Fields[parts[2]]
			for i, candidate := range after.Items {
				if candidate.ItemID == item.ItemID {
					label = fmt.Sprintf("Позиция %d · %s", i+1, field.Label)
				}
			}
		}
		if field.Label == "" {
			continue
		}
		value := "не указано"
		switch v := change.After.(type) {
		case string:
			value = v
			if translated := field.ValueLabels[v]; translated != "" {
				value = translated
			}
		case float64:
			value = strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			value = "нет"
			if v {
				value = "да"
			}
		}
		if runes := []rune(value); len(runes) > 80 {
			value = string(runes[:80]) + "…"
		}
		line := label + ": " + value
		if change.Path == "header.prepayment" {
			lines = append([]string{line}, lines...)
		} else {
			lines = append(lines, line)
		}
	}
	if len(lines) > 3 {
		lines = append(lines[:3], fmt.Sprintf("И ещё изменений: %d", len(lines)-3))
	}
	for i := range lines {
		lines[i] = "• " + lines[i]
	}
	return lines
}
