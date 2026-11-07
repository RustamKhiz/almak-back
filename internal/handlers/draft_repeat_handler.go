package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"almak-back/internal/aiorders"
	"almak-back/internal/draftorders"
	"almak-back/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (h *DraftHandler) Repeat(c *gin.Context) {
	var req struct {
		RequestID    string `json:"requestId"`
		CopyComment  bool   `json:"copyComment"`
		CopyDiscount bool   `json:"copyDiscount"`
	}
	if !decodeDraft(c, &req) {
		return
	}
	source, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize)
	if err != nil || source == 0 || !draftorders.Identifier.MatchString(req.RequestID) {
		c.JSON(400, gin.H{"message": "Некорректный запрос на повтор заказа"})
		return
	}
	user := c.MustGet("user").(models.User)
	if req.CopyDiscount && !user.Can(models.DiscountsWrite) {
		c.JSON(403, gin.H{"message": "Недостаточно прав для переноса скидки"})
		return
	}
	actor := user.ID
	key := fmt.Sprintf("repeat:%d:%d:%t:%t", actor, source, req.CopyComment, req.CopyDiscount)
	db := draftDB().WithContext(c.Request.Context())
	existing := func() (bool, error) {
		var row models.OrderDraft
		err := db.Select("create_request").First(&row, "id = ?", req.RequestID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if row.CreateRequest != key {
			c.JSON(409, gin.H{"message": "Этот запрос уже использован"})
		} else {
			respondDraft(c, req.RequestID)
		}
		return true, nil
	}
	if found, err := existing(); found || err != nil {
		if err != nil {
			draftError(c, err, req.RequestID)
		}
		return
	}

	var order models.Order
	err = db.Transaction(func(tx *gorm.DB) error {
		return preloadOrder(tx).First(&order, source).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"message": "Исходный заказ не найден"})
		return
	}
	if err != nil {
		draftError(c, err, req.RequestID)
		return
	}
	payload, err := repeatPayload(order, req.CopyComment, req.CopyDiscount)
	if err != nil || draftorders.ValidatePayload(payload) != nil {
		c.JSON(422, gin.H{"message": "Позиции исходного заказа не удалось перенести в черновик"})
		return
	}
	row := models.OrderDraft{
		ID: req.RequestID, Kind: "manual", Revision: 1, State: "active",
		Document: draftorders.Document{Payload: payload}, SourceOrderID: &order.ID,
		CreatedBy: &actor, UpdatedBy: actor, CreateRequest: key,
	}
	result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		draftError(c, result.Error, req.RequestID)
		return
	}
	if result.RowsAffected == 0 {
		if found, err := existing(); !found {
			draftError(c, err, req.RequestID)
		}
		return
	}
	respondDraft(c, row.ID)
}

func repeatPayload(order models.Order, copyComment, copyDiscount bool) (json.RawMessage, error) {
	raw, err := json.Marshal(order)
	if err != nil {
		return nil, err
	}
	var original map[string]json.RawMessage
	if err := json.Unmarshal(raw, &original); err != nil {
		return nil, err
	}
	payload := map[string]any{
		"name": order.Customer, "phone": order.Phone, "date": "", "status": 1,
		"needsDelivery": order.NeedsDelivery, "deliveryAddress": order.DeliveryAddress,
		"comment": "", "discount": 0, "prepayment": 0, "isPaid": false, "payments": []any{},
	}
	if copyComment {
		payload["comment"] = order.Comment
	}
	if copyDiscount {
		payload["discount"] = order.Discount
	}
	for kind, definition := range aiorders.Definitions.Items {
		items := []map[string]any{}
		if data := original[definition.Collection]; len(data) > 0 {
			if err := json.Unmarshal(data, &items); err != nil {
				return nil, err
			}
		}
		for i, item := range items {
			delete(item, "order_id")
			item["id"], item["type"], item["costPrice"] = i+1, kind, 0
			for field, rule := range definition.Fields {
				if len(rule.Options) > 0 {
					value, _ := item[field].(string)
					if !slices.Contains(rule.Options, value) {
						item[field] = rule.Options[0]
					}
				}
			}
			switch kind {
			case "extension":
				product := order.Extensions[i]
				item["sizes"] = normalizeExtensionSizes(extensionRequest{
					Sizes: product.Sizes, Width: product.Width, Height: product.Height,
					QuantityPerSet: product.QuantityPerSet,
				})
			case "paneling":
				delete(item, "quantityPerSet")
				product := order.Panelings[i]
				sizes := []models.Size{}
				for _, part := range strings.Split(product.Size, ";") {
					var width, height int
					part = strings.NewReplacer("х", "x", "×", "x", " ", "").Replace(strings.ToLower(part))
					if n, _ := fmt.Sscanf(part, "%dx%d", &width, &height); n == 2 && width > 0 && height > 0 {
						sizes = append(sizes, models.Size{Width: width, Height: height})
					}
				}
				if len(sizes) <= 1 && len(product.Sizes) > 0 {
					sizes = product.Sizes
				}
				if len(sizes) == 0 {
					sizes = []models.Size{{Width: product.Width, Height: product.Height}}
				}
				item["sizes"] = sizes
				if len(product.Sizes) == 0 && product.TotalArea > 0 {
					item["totalArea"] = roundMoney(product.TotalArea * math.Max(float64(product.Count), 1))
				} else {
					item["totalArea"] = normalizePanelingTotalArea(panelingRequest{Sizes: sizes})
				}
			case "hardware":
				for _, field := range []string{"handleModel", "handleColor"} {
					if item[field] == nil {
						item[field] = ""
					}
				}
				for _, field := range []string{"hingeRightCount", "hingeLeftCount"} {
					if item[field] == nil {
						item[field] = item["hingeCount"]
					}
				}
			}
		}
		payload[definition.Collection] = items
	}
	return json.Marshal(payload)
}
