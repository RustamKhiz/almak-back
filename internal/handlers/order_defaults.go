package handlers

import (
	"almak-back/internal/aiorders"
	"time"
)

func applyOrderDefaults(mode string, before aiorders.Order, order *aiorders.Order, now time.Time) {
	if mode != "edit_order" && (order.Header["date"] == nil || order.Header["date"] == "") {
		order.Header["date"] = now.In(time.FixedZone("MSK", 3*60*60)).Format("2006-01-02")
	}
	for i := range order.Items {
		item := &order.Items[i]
		if item.Type != "hardware" {
			continue
		}
		unchanged := false
		if mode == "edit_order" {
			for _, old := range before.Items {
				if old.ItemID == item.ItemID && old.Fields["lockCount"] == item.Fields["lockCount"] {
					unchanged = true
				}
			}
		}
		if unchanged {
			continue
		}
		count, ok := item.Fields["lockCount"].(float64)
		if !ok || count <= 0 {
			continue
		}
		for _, key := range []string{"cylinderCount", "escutcheonCount"} {
			if item.Fields[key] == nil {
				item.Fields[key] = count
			}
		}
	}
}
