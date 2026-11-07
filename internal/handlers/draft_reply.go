package handlers

import (
	"fmt"
	"strings"

	"almak-back/internal/aiorders"
)

func draftReply(mode string, order aiorders.Order, changes int) string {
	text := "Черновик создан."
	if mode == "refine" {
		text = "Черновик обновлён."
	}
	if mode == "edit_order" {
		text = "Изменения подготовлены. Проверьте «было → стало» перед применением."
		if changes == 0 {
			text = "В рабочей копии пока нет изменений."
		}
	} else {
		text += fmt.Sprintf(" Позиций: %d.", len(order.Items))
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
		text += "\n\nДля проверки:\n• " + strings.Join(order.Notes, "\n• ")
	}
	return text
}
