package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"almak-back/internal/aiorders"
	"almak-back/internal/draftorders"
	"almak-back/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func canAccessDraft(c *gin.Context, row models.OrderDraft) bool {
	if row.TargetOrderID == nil {
		return true
	}
	user := c.MustGet("user").(models.User)
	return row.CreatedBy != nil && *row.CreatedBy == user.ID &&
		user.Can(models.OrdersRead) && user.Can(models.OrdersWrite) && user.Can(models.AssistantUse)
}

func (h *DraftHandler) StartOrderEdit(c *gin.Context) {
	orderID, ok := parseID(c)
	if !ok {
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if !decodeDraft(c, &req) {
		return
	}
	if !draftorders.Identifier.MatchString(req.ID) {
		c.JSON(400, gin.H{"message": "Некорректный ID рабочей копии"})
		return
	}
	actor := draftActor(c)
	var row models.OrderDraft
	err := draftDB().WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var order models.Order
		if err := preloadOrder(tx.Clauses(clause.Locking{Strength: "UPDATE"})).First(&order, orderID).Error; err != nil {
			return err
		}
		err := tx.Where("target_order_id = ? AND base_order_revision = ? AND created_by = ? AND state = ?", orderID, order.Revision, actor, "active").Order("created_at DESC").First(&row).Error
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		base, err := editableOrder(order)
		if err != nil {
			return err
		}
		row = models.OrderDraft{
			ID: req.ID, Kind: "ai", State: "active", Revision: 1, CreatedBy: &actor, UpdatedBy: actor,
			TargetOrderID: &orderID, BaseOrderRevision: order.Revision, EditBase: &base,
			Document: draftorders.Document{Order: &base, FieldSources: map[string]aiorders.Source{}, Changes: []aiorders.Change{}, Questions: []string{}},
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"message": "Заказ не найден"})
		} else {
			c.JSON(422, gin.H{"message": "Не удалось подготовить рабочую копию заказа"})
		}
		return
	}
	respondDraft(c, row.ID)
}

func editableOrder(order models.Order) (aiorders.Order, error) {
	result := aiorders.Order{
		Header: map[string]any{"name": order.Customer, "phone": order.Phone, "comment": order.Comment,
			"needsDelivery": order.NeedsDelivery, "deliveryAddress": order.DeliveryAddress},
		Items: []aiorders.Item{}, Notes: []string{},
	}
	data, err := json.Marshal(order)
	if err != nil {
		return result, err
	}
	var collections map[string]json.RawMessage
	if err := json.Unmarshal(data, &collections); err != nil {
		return result, err
	}
	// Keep the same category order as the order card.
	for _, kind := range []string{"entranceDoor", "interiorDoor", "molding", "extension", "capital", "hardware", "paneling", "skirting"} {
		def := aiorders.Definitions.Items[kind]
		raw := collections[def.Collection]
		if len(raw) == 0 {
			continue
		}
		var items []map[string]any
		if err := json.Unmarshal(raw, &items); err != nil {
			return result, err
		}
		for _, record := range items {
			item := aiorders.Item{ItemID: editItemID(kind, uint(record["id"].(float64))), Type: kind, Fields: map[string]any{}}
			for field, rule := range def.Fields {
				value := record[field]
				if protectedEditField(field) || value == nil {
					continue
				}
				if len(rule.Options) > 0 {
					option, _ := value.(string)
					if !slices.Contains(rule.Options, option) {
						continue
					}
				}
				item.Fields[field] = value
			}
			result.Items = append(result.Items, item)
		}
	}
	return result, aiorders.ValidateOrder(result)
}

func editItemID(kind string, id uint) string { return kind + "-" + strconv.FormatUint(uint64(id), 10) }
func protectedEditField(field string) bool   { return field == "supplier" || field == "costPrice" }

func normalizeEditProposal(base, before aiorders.Order, after *aiorders.Order) bool {
	for field := range after.Header {
		if _, allowed := before.Header[field]; !allowed {
			delete(after.Header, field)
		}
	}
	for field, value := range before.Header {
		if after.Header[field] == nil {
			after.Header[field] = value
		}
	}
	// Address remains visible in the preview when switching to pickup.
	if after.Header["needsDelivery"] == false && before.Header["needsDelivery"] == true {
		after.Header["deliveryAddress"] = ""
	}
	byID := make(map[string]aiorders.Item, len(before.Items))
	rank := make(map[string]int, len(before.Items))
	for i, item := range before.Items {
		byID[item.ItemID] = item
		rank[item.ItemID] = i + 1
	}
	sort.SliceStable(after.Items, func(i, j int) bool {
		left, right := rank[after.Items[i].ItemID], rank[after.Items[j].ItemID]
		return left != 0 && (right == 0 || left < right)
	})
	registered := make(map[string]bool, len(base.Items))
	for _, item := range base.Items {
		registered[item.ItemID] = true
	}
	sizesRejected := false
	for _, item := range after.Items {
		previous := byID[item.ItemID].Fields
		delete(item.Fields, "supplier")
		delete(item.Fields, "costPrice")
		for field, value := range previous {
			if item.Fields[field] == nil {
				item.Fields[field] = value
			}
		}
		if item.Type == "hardware" &&
			(item.Fields["hingeLeftCount"] != previous["hingeLeftCount"] || item.Fields["hingeRightCount"] != previous["hingeRightCount"]) {
			left, _ := item.Fields["hingeLeftCount"].(float64)
			right, _ := item.Fields["hingeRightCount"].(float64)
			item.Fields["hingeCount"] = left + right
		}
		if item.Type == "molding" &&
			(item.Fields["frameSetCount"] != previous["frameSetCount"] || item.Fields["frameBoxCount"] != previous["frameBoxCount"]) {
			sets, _ := item.Fields["frameSetCount"].(float64)
			boxes, _ := item.Fields["frameBoxCount"].(float64)
			item.Fields["frameCount"] = normalizeFrameCount(sets, boxes)
		}
		if registered[item.ItemID] {
			for field, value := range item.Fields {
				if lockedSizeField(item.Type, field) && !reflect.DeepEqual(value, byID[item.ItemID].Fields[field]) {
					sizesRejected = true
					if previous, ok := byID[item.ItemID].Fields[field]; ok {
						item.Fields[field] = previous
					} else {
						delete(item.Fields, field)
					}
				}
			}
		}
	}
	return sizesRejected
}

func lockedSizeField(kind, field string) bool {
	return (kind == "paneling" || kind == "extension") &&
		(field == "width" || field == "height" || field == "quantityPerSet" || field == "count")
}

func editChanges(before, after aiorders.Order) []aiorders.Change {
	before.Notes, after.Notes = nil, nil
	changes := aiorders.Changes(before, after)
	labels := map[string]string{}
	for i, item := range before.Items {
		labels[item.ItemID] = fmt.Sprintf("Позиция %d", i+1)
	}
	for i, item := range after.Items {
		if labels[item.ItemID] == "" {
			labels[item.ItemID] = fmt.Sprintf("Новая позиция %d", i+1)
		}
	}
	for i := range changes {
		parts := strings.Split(changes[i].Path, ".")
		if len(parts) == 3 && parts[0] == "items" {
			changes[i].Label = labels[parts[1]] + " · " + changes[i].Label
		}
	}
	return changes
}

type invalidOrderEdit string

func (err invalidOrderEdit) Error() string { return string(err) }

func (h *DraftHandler) ApplyOrderEdit(c *gin.Context) {
	var req struct {
		BaseRevision int `json:"baseRevision"`
	}
	if !decodeDraft(c, &req) {
		return
	}
	id := c.Param("id")
	if !draftorders.Identifier.MatchString(id) || req.BaseRevision < 1 {
		c.JSON(400, gin.H{"message": "Некорректная версия рабочей копии"})
		return
	}
	err := draftDB().WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var row models.OrderDraft
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error; err != nil {
			return err
		}
		if row.TargetOrderID == nil || !canAccessDraft(c, row) {
			return gorm.ErrRecordNotFound
		}
		if row.State == "converted" && row.OrderID != nil {
			return nil
		}
		if row.State != "active" || row.Revision != req.BaseRevision || row.EditBase == nil || row.Document.Order == nil {
			return errDraftConflict
		}
		var working int64
		if err := tx.Model(&models.DraftAIRequest{}).Where("draft_id = ? AND status = ? AND updated_at > ?", id, "working", time.Now().Add(-time.Minute)).Count(&working).Error; err != nil {
			return err
		}
		if working > 0 {
			return errDraftBusy
		}
		var order models.Order
		if err := preloadOrder(tx.Clauses(clause.Locking{Strength: "UPDATE"})).First(&order, *row.TargetOrderID).Error; err != nil {
			return err
		}
		if order.Revision != row.BaseOrderRevision {
			return errOrderRevision
		}
		if len(row.Document.Changes) == 0 || len(row.Document.Order.Items) == 0 {
			return invalidOrderEdit("Нужны изменения и хотя бы одна товарная позиция")
		}
		if strings.TrimSpace(row.Document.PendingText) != "" {
			return invalidOrderEdit("Отправьте или очистите новое уточнение перед применением.")
		}
		if err := applyEditProposal(tx, &order, *row.EditBase, *row.Document.Order); err != nil {
			return err
		}
		row.Document.Questions = []string{}
		row.Document.LastAppliedChange = nil
		row.State = "converted"
		row.OrderID = &order.ID
		row.Revision++
		row.UpdatedBy = draftActor(c)
		if err := tx.Model(&row).Select("state", "order_id", "revision", "updated_by", "document", "updated_at").Updates(&row).Error; err != nil {
			return err
		}
		return tx.Create(&models.DraftMessage{DraftID: id, Role: "assistant", Text: fmt.Sprintf("Изменения применены к заказу №%d. Версия заказа: %d → %d.", order.ID, order.Revision, order.Revision+1)}).Error
	})
	var invalid invalidOrderEdit
	switch {
	case errors.Is(err, errOrderRevision):
		c.JSON(409, gin.H{"code": "ORDER_CONFLICT", "message": "Заказ уже изменился. Рабочая копия сохранена. Откройте карточку и начните правку с актуальной версии."})
	case errors.As(err, &invalid):
		c.JSON(422, gin.H{"message": invalid.Error()})
	case err != nil:
		draftError(c, err, id)
	default:
		respondDraft(c, id)
	}
}

func applyEditProposal(tx *gorm.DB, order *models.Order, base, proposal aiorders.Order) error {
	// Check newly introduced issues without blocking edits to unrelated legacy fields.
	baseIssues := map[aiorders.Issue]bool{}
	for _, issue := range aiorders.Issues(base) {
		baseIssues[issue] = true
	}
	for _, issue := range aiorders.Issues(proposal) {
		if issue.Path == "header.date" && proposal.Header["date"] == nil {
			continue
		}
		if !baseIssues[issue] {
			return invalidOrderEdit(issue.Message)
		}
	}
	before := make(map[string]aiorders.Item, len(base.Items))
	for _, item := range base.Items {
		before[item.ItemID] = item
	}
	for _, item := range proposal.Items {
		if original, ok := before[item.ItemID]; ok && original.Type != item.Type {
			return invalidOrderEdit("Чтобы заменить тип товара, удалите позицию и добавьте новую.")
		}
	}
	for _, run := range []func() error{
		func() error {
			return editItems(tx, order.ID, "interiorDoor", &order.InteriorDoors, before, proposal.Items)
		},
		func() error {
			return editItems(tx, order.ID, "entranceDoor", &order.EntranceDoors, before, proposal.Items)
		},
		func() error { return editItems(tx, order.ID, "molding", &order.Moldings, before, proposal.Items) },
		func() error { return editItems(tx, order.ID, "extension", &order.Extensions, before, proposal.Items) },
		func() error { return editItems(tx, order.ID, "capital", &order.Capitals, before, proposal.Items) },
		func() error { return editItems(tx, order.ID, "hardware", &order.Hardwares, before, proposal.Items) },
		func() error { return editItems(tx, order.ID, "paneling", &order.Panelings, before, proposal.Items) },
		func() error { return editItems(tx, order.ID, "skirting", &order.Skirtings, before, proposal.Items) },
	} {
		if err := run(); err != nil {
			return err
		}
	}
	columns := map[string]any{"requires_revision": true}
	for field, column := range map[string]string{"name": "customer", "phone": "phone", "comment": "comment", "needsDelivery": "needs_delivery", "deliveryAddress": "delivery_address"} {
		if proposal.Header[field] != base.Header[field] {
			columns[column] = proposal.Header[field]
		}
	}
	if len(aiorders.Changes(
		aiorders.Order{Items: base.Items}, aiorders.Order{Items: proposal.Items},
	)) > 0 {
		raw, err := json.Marshal(order)
		if err != nil {
			return err
		}
		var pricing orderRequest
		if err := json.Unmarshal(raw, &pricing); err != nil {
			return err
		}
		columns["price"] = calculateOrderPrice(pricing)
		if err := tx.Model(order).Updates(columns).Error; err != nil {
			return err
		}
		return syncOrderPrepayment(tx, order.ID, true)
	}
	columns["revision"] = gorm.Expr("revision + 1")
	return tx.Model(order).Updates(columns).Error
}

func editItems[T any](tx *gorm.DB, orderID uint, kind string, records *[]T, before map[string]aiorders.Item, proposal []aiorders.Item) error {
	pending := map[string]aiorders.Item{}
	for _, item := range proposal {
		if item.Type == kind {
			pending[item.ItemID] = item
		}
	}
	next := make([]T, 0, len(pending))
	for _, record := range *records {
		raw, err := json.Marshal(record)
		if err != nil {
			return err
		}
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			return err
		}
		id := uint(fields["id"].(float64))
		key := editItemID(kind, id)
		item, keep := pending[key]
		if !keep {
			if err := tx.Delete(&record, id).Error; err != nil {
				return err
			}
			continue
		}
		delete(pending, key)
		changed := false
		for field, value := range item.Fields {
			if !reflect.DeepEqual(value, before[key].Fields[field]) {
				if lockedSizeField(kind, field) {
					return invalidOrderEdit("Размеры и количество доборов и обшивки изменяйте в обычном редакторе.")
				}
				fields[field], changed = value, true
			}
		}
		if changed {
			if err := decodeEditItem(fields, &record); err != nil {
				return err
			}
			if err := tx.Save(&record).Error; err != nil {
				return err
			}
		}
		next = append(next, record)
	}
	for _, item := range proposal {
		if _, add := pending[item.ItemID]; !add {
			continue
		}
		fields := map[string]any{"order_id": orderID}
		for key, value := range item.Fields {
			fields[key] = value
		}
		var record T
		if err := decodeEditItem(fields, &record); err != nil {
			return err
		}
		switch product := any(&record).(type) {
		case *models.Extension:
			product.QuantityPerSet *= product.Count
			product.Count = 1
			product.Sizes = models.ExtensionSizes(normalizeExtensionSizes(extensionRequest{Width: product.Width, Height: product.Height, QuantityPerSet: product.QuantityPerSet}))
			product.TotalArea = calculateExtensionAutoTotalArea(product.Sizes)
		case *models.Paneling:
			// A newly added row represents one group of identical panels.
			if product.Count < 1 || product.Count > 100 {
				return invalidOrderEdit("Укажите от 1 до 100 панелей в новой позиции.")
			}
			product.Sizes = make(models.Sizes, product.Count)
			for i := range product.Sizes {
				product.Sizes[i] = models.Size{Width: product.Width, Height: product.Height}
			}
			product.Size = formatSizes(product.Sizes)
			product.TotalArea = normalizePanelingTotalArea(panelingRequest{Sizes: product.Sizes})
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		next = append(next, record)
	}
	*records = next
	return nil
}

func decodeEditItem(fields map[string]any, record any) error {
	raw, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, record)
}
