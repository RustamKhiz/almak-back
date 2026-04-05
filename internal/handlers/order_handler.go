package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"almak-back/internal/database"
	"almak-back/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type OrderHandler struct{}

type interiorDoorRequest struct { Model string `json:"model" binding:"required"`; Price float64 `json:"price" binding:"required"`; Width int `json:"width" binding:"required"`; Width2 *int `json:"width2"`; Height int `json:"height" binding:"required"`; HasGlass bool `json:"hasGlass"`; LeafType string `json:"leafType" binding:"required"`; Count int `json:"count" binding:"required"`; Covering string `json:"covering" binding:"required"`; Comment string `json:"comment"` }
type entranceDoorRequest struct { Kind string `json:"kind" binding:"required"`; Model string `json:"model" binding:"required"`; Width int `json:"width" binding:"required"`; Height int `json:"height" binding:"required"`; Color string `json:"color" binding:"required"`; Painting *string `json:"painting"`; PanelColor *string `json:"panelColor"`; HasPeephole *bool `json:"hasPeephole"`; Count int `json:"count" binding:"required"`; Price float64 `json:"price" binding:"required"`; Comment string `json:"comment"` }
type moldingRequest struct { FrameLength *int `json:"frameLength"`; FramePrice float64 `json:"framePrice" binding:"required"`; FrameCount int `json:"frameCount" binding:"required"`; PlatbandType string `json:"platbandType" binding:"required"`; PlatbandFigure *string `json:"platbandFigure"`; PlatbandLength *int `json:"platbandLength"`; PlatbandPrice float64 `json:"platbandPrice" binding:"required"`; PlatbandCount int `json:"platbandCount" binding:"required"`; RebateBarCount int `json:"rebateBarCount"`; Color string `json:"color" binding:"required"`; Covering string `json:"covering" binding:"required"`; Comment string `json:"comment"` }
type extensionRequest struct { Color string `json:"color" binding:"required"`; Covering string `json:"covering" binding:"required"`; Width int `json:"width" binding:"required"`; Height int `json:"height" binding:"required"`; Comment string `json:"comment"`; Count int `json:"count" binding:"required"`; Price float64 `json:"price" binding:"required"` }
type capitalRequest struct { Name string `json:"name" binding:"required"`; Color string `json:"color" binding:"required"`; Covering string `json:"covering" binding:"required"`; Width int `json:"width" binding:"required"`; Height int `json:"height" binding:"required"`; Comment string `json:"comment"`; Count int `json:"count" binding:"required"` }
type panelingRequest struct { Color string `json:"color" binding:"required"`; Size string `json:"size" binding:"required"`; Covering string `json:"covering" binding:"required"`; Count int `json:"count" binding:"required"`; Price float64 `json:"price" binding:"required"`; Comment string `json:"comment"` }

type orderRequest struct {
	Customer string `json:"customer" binding:"required"`
	Phone string `json:"phone" binding:"required"`
	Date string `json:"date" binding:"required"`
	Prepayment float64 `json:"prepayment" binding:"required"`
	Discount float64 `json:"discount"`
	NeedsDelivery bool `json:"needsDelivery"`
	DeliveryAddress string `json:"deliveryAddress"`
	Comment string `json:"comment"`
	Status string `json:"status" binding:"required"`
	InteriorDoors []interiorDoorRequest `json:"interiorDoors"`
	EntranceDoors []entranceDoorRequest `json:"entranceDoors"`
	Moldings []moldingRequest `json:"moldings"`
	Extensions []extensionRequest `json:"extensions"`
	Capitals []capitalRequest `json:"capitals"`
	Panelings []panelingRequest `json:"panelings"`
}

type orderStatusRequest struct { Status int `json:"status" binding:"required"` }

func NewOrderHandler() *OrderHandler { return &OrderHandler{} }

func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req orderRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"}); return }
	if !hasOrderItems(req) { c.JSON(http.StatusBadRequest, gin.H{"error": "order must contain at least one item"}); return }
	if req.NeedsDelivery && strings.TrimSpace(req.DeliveryAddress) == "" { c.JSON(http.StatusBadRequest, gin.H{"error": "delivery address is required"}); return }
	order := models.Order{ Customer: req.Customer, Phone: req.Phone, Date: req.Date, Price: calculateOrderPrice(req), Prepayment: req.Prepayment, Discount: req.Discount, NeedsDelivery: req.NeedsDelivery, DeliveryAddress: normalizeDeliveryAddress(req.NeedsDelivery, req.DeliveryAddress), Comment: req.Comment, Status: req.Status, InteriorDoors: mapInteriorDoorsForCreate(req.InteriorDoors), EntranceDoors: mapEntranceDoorsForCreate(req.EntranceDoors), Moldings: mapMoldingsForCreate(req.Moldings), Extensions: mapExtensionsForCreate(req.Extensions), Capitals: mapCapitalsForCreate(req.Capitals), Panelings: mapPanelingsForCreate(req.Panelings) }
	if err := database.DB.Create(&order).Error; err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order"}); return }
	if err := preloadOrder(database.DB).First(&order, order.ID).Error; err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load created order"}); return }
	c.JSON(http.StatusCreated, order)
}

func (h *OrderHandler) GetOrders(c *gin.Context) { var orders []models.Order; if err := database.DB.Order("id DESC").Find(&orders).Error; err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load orders"}); return }; c.JSON(http.StatusOK, orders) }
func (h *OrderHandler) GetOrderByID(c *gin.Context) { id, ok := parseID(c); if !ok { return }; var order models.Order; if err := preloadOrder(database.DB).First(&order, id).Error; err != nil { c.JSON(http.StatusNotFound, gin.H{"error": "order not found"}); return }; c.JSON(http.StatusOK, order) }

func (h *OrderHandler) UpdateOrder(c *gin.Context) {
	id, ok := parseID(c); if !ok { return }
	var req orderRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"}); return }
	if !hasOrderItems(req) { c.JSON(http.StatusBadRequest, gin.H{"error": "order must contain at least one item"}); return }
	if req.NeedsDelivery && strings.TrimSpace(req.DeliveryAddress) == "" { c.JSON(http.StatusBadRequest, gin.H{"error": "delivery address is required"}); return }
	var order models.Order
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&order, id).Error; err != nil { return err }
		order.Customer = req.Customer; order.Phone = req.Phone; order.Date = req.Date; order.Price = calculateOrderPrice(req); order.Prepayment = req.Prepayment; order.Discount = req.Discount; order.NeedsDelivery = req.NeedsDelivery; order.DeliveryAddress = normalizeDeliveryAddress(req.NeedsDelivery, req.DeliveryAddress); order.Comment = req.Comment; order.Status = req.Status
		if err := tx.Save(&order).Error; err != nil { return err }
		if err := tx.Where("order_id = ?", order.ID).Delete(&models.InteriorDoor{}).Error; err != nil { return err }
		if err := tx.Where("order_id = ?", order.ID).Delete(&models.EntranceDoor{}).Error; err != nil { return err }
		if err := tx.Where("order_id = ?", order.ID).Delete(&models.Molding{}).Error; err != nil { return err }
		if err := tx.Where("order_id = ?", order.ID).Delete(&models.Extension{}).Error; err != nil { return err }
		if err := tx.Where("order_id = ?", order.ID).Delete(&models.Capital{}).Error; err != nil { return err }
		if err := tx.Where("order_id = ?", order.ID).Delete(&models.Paneling{}).Error; err != nil { return err }
		interiorDoors := mapInteriorDoorsForCreate(req.InteriorDoors); for i := range interiorDoors { interiorDoors[i].OrderID = order.ID }; if len(interiorDoors) > 0 { if err := tx.Create(&interiorDoors).Error; err != nil { return err } }
		entranceDoors := mapEntranceDoorsForCreate(req.EntranceDoors); for i := range entranceDoors { entranceDoors[i].OrderID = order.ID }; if len(entranceDoors) > 0 { if err := tx.Create(&entranceDoors).Error; err != nil { return err } }
		moldings := mapMoldingsForCreate(req.Moldings); for i := range moldings { moldings[i].OrderID = order.ID }; if len(moldings) > 0 { if err := tx.Create(&moldings).Error; err != nil { return err } }
		extensions := mapExtensionsForCreate(req.Extensions); for i := range extensions { extensions[i].OrderID = order.ID }; if len(extensions) > 0 { if err := tx.Create(&extensions).Error; err != nil { return err } }
		capitals := mapCapitalsForCreate(req.Capitals); for i := range capitals { capitals[i].OrderID = order.ID }; if len(capitals) > 0 { if err := tx.Create(&capitals).Error; err != nil { return err } }
		panelings := mapPanelingsForCreate(req.Panelings); for i := range panelings { panelings[i].OrderID = order.ID }; if len(panelings) > 0 { if err := tx.Create(&panelings).Error; err != nil { return err } }
		return nil
	})
	if err != nil { if errors.Is(err, gorm.ErrRecordNotFound) { c.JSON(http.StatusNotFound, gin.H{"error": "order not found"}); return }; c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update order"}); return }
	if err := preloadOrder(database.DB).First(&order, id).Error; err != nil { c.JSON(http.StatusNotFound, gin.H{"error": "order not found"}); return }
	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) DeleteOrder(c *gin.Context) { id, ok := parseID(c); if !ok { return }; result := database.DB.Delete(&models.Order{}, id); if result.Error != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete order"}); return }; if result.RowsAffected == 0 { c.JSON(http.StatusNotFound, gin.H{"error": "order not found"}); return }; c.JSON(http.StatusOK, gin.H{"message": "order deleted"}) }
func (h *OrderHandler) UpdateOrderStatus(c *gin.Context) {
	id, ok := parseID(c); if !ok { return }
	var req orderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"}); return }
	status, statusOk := statusCodeToValue(req.Status); if !statusOk { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"}); return }
	var order models.Order
	if err := database.DB.First(&order, id).Error; err != nil { if errors.Is(err, gorm.ErrRecordNotFound) { c.JSON(http.StatusNotFound, gin.H{"error": "order not found"}); return }; c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update order status"}); return }
	if err := database.DB.Model(&order).Update("status", status).Error; err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update order status"}); return }
	if err := preloadOrder(database.DB).First(&order, id).Error; err != nil { c.JSON(http.StatusNotFound, gin.H{"error": "order not found"}); return }
	c.JSON(http.StatusOK, order)
}

func parseID(c *gin.Context) (uint, bool) { id, err := strconv.ParseUint(c.Param("id"), 10, 64); if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"}); return 0, false }; return uint(id), true }
func calculateOrderPrice(req orderRequest) float64 { total := 0.0; for _, door := range req.InteriorDoors { total += door.Price * float64(door.Count) }; for _, door := range req.EntranceDoors { total += door.Price * float64(door.Count) }; for _, item := range req.Moldings { total += item.FramePrice*float64(item.FrameCount) + item.PlatbandPrice*float64(item.PlatbandCount) }; for _, item := range req.Extensions { total += item.Price * float64(item.Count) }; for _, item := range req.Panelings { total += item.Price * float64(item.Count) }; return total }
func statusCodeToValue(status int) (string, bool) { switch status { case 1: return "accepted", true; case 2: return "progress", true; case 3: return "completed", true; default: return "", false } }
func mapInteriorDoorsForCreate(doors []interiorDoorRequest) []models.InteriorDoor { result := make([]models.InteriorDoor, 0, len(doors)); for _, door := range doors { result = append(result, models.InteriorDoor{ Model: door.Model, Price: door.Price, Width: door.Width, Width2: door.Width2, Height: door.Height, HasGlass: door.HasGlass, LeafType: door.LeafType, Count: door.Count, Covering: door.Covering, Comment: strings.TrimSpace(door.Comment) }) }; return result }
func mapEntranceDoorsForCreate(doors []entranceDoorRequest) []models.EntranceDoor { result := make([]models.EntranceDoor, 0, len(doors)); for _, door := range doors { result = append(result, models.EntranceDoor{ Kind: strings.TrimSpace(door.Kind), Model: strings.TrimSpace(door.Model), Width: door.Width, Height: door.Height, Color: strings.TrimSpace(door.Color), Painting: normalizeOptionalString(door.Painting), PanelColor: normalizeOptionalString(door.PanelColor), HasPeephole: door.HasPeephole, Count: door.Count, Price: door.Price, Comment: strings.TrimSpace(door.Comment) }) }; return result }
func mapMoldingsForCreate(items []moldingRequest) []models.Molding { result := make([]models.Molding, 0, len(items)); for _, item := range items { result = append(result, models.Molding{ FrameLength: normalizeOptionalInt(item.FrameLength), FramePrice: item.FramePrice, FrameCount: item.FrameCount, PlatbandType: strings.TrimSpace(item.PlatbandType), PlatbandFigure: normalizeOptionalString(item.PlatbandFigure), PlatbandLength: normalizeOptionalInt(item.PlatbandLength), PlatbandPrice: item.PlatbandPrice, PlatbandCount: item.PlatbandCount, RebateBarCount: item.RebateBarCount, Color: strings.TrimSpace(item.Color), Covering: strings.TrimSpace(item.Covering), Comment: strings.TrimSpace(item.Comment) }) }; return result }
func mapExtensionsForCreate(items []extensionRequest) []models.Extension { result := make([]models.Extension, 0, len(items)); for _, item := range items { result = append(result, models.Extension{ Color: strings.TrimSpace(item.Color), Covering: strings.TrimSpace(item.Covering), Width: item.Width, Height: item.Height, Comment: strings.TrimSpace(item.Comment), Count: item.Count, Price: item.Price }) }; return result }
func mapCapitalsForCreate(items []capitalRequest) []models.Capital { result := make([]models.Capital, 0, len(items)); for _, item := range items { result = append(result, models.Capital{ Name: strings.TrimSpace(item.Name), Color: strings.TrimSpace(item.Color), Covering: strings.TrimSpace(item.Covering), Width: item.Width, Height: item.Height, Comment: strings.TrimSpace(item.Comment), Count: item.Count }) }; return result }
func mapPanelingsForCreate(items []panelingRequest) []models.Paneling { result := make([]models.Paneling, 0, len(items)); for _, item := range items { result = append(result, models.Paneling{ Color: strings.TrimSpace(item.Color), Size: strings.TrimSpace(item.Size), Covering: strings.TrimSpace(item.Covering), Count: item.Count, Price: item.Price, Comment: strings.TrimSpace(item.Comment) }) }; return result }
func normalizeDeliveryAddress(needsDelivery bool, address string) string { if !needsDelivery { return "" }; return strings.TrimSpace(address) }
func normalizeOptionalString(value *string) *string { if value == nil { return nil }; normalized := strings.TrimSpace(*value); if normalized == "" { return nil }; return &normalized }
func normalizeOptionalInt(value *int) *int { if value == nil { return nil }; normalized := *value; if normalized <= 0 { return nil }; return &normalized }
func hasOrderItems(req orderRequest) bool { return len(req.InteriorDoors) > 0 || len(req.EntranceDoors) > 0 || len(req.Moldings) > 0 || len(req.Extensions) > 0 || len(req.Capitals) > 0 || len(req.Panelings) > 0 }
func preloadOrder(db *gorm.DB) *gorm.DB { return db.Preload("InteriorDoors").Preload("EntranceDoors").Preload("Moldings").Preload("Extensions").Preload("Capitals").Preload("Panelings") }
