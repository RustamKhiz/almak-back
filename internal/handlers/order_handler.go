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

type doorRequest struct {
	Model    string  `json:"model" binding:"required"`
	Price    float64 `json:"price" binding:"required"`
	Width    int     `json:"width" binding:"required"`
	Width2   *int    `json:"width2"`
	Height   int     `json:"height" binding:"required"`
	HasGlass bool    `json:"hasGlass"`
	LeafType string  `json:"leafType" binding:"required"`
	Count    int     `json:"count" binding:"required"`
	Comment  string  `json:"comment"`
}

type orderRequest struct {
	Customer        string        `json:"customer" binding:"required"`
	Phone           string        `json:"phone" binding:"required"`
	Date            string        `json:"date" binding:"required"`
	Prepayment      float64       `json:"prepayment" binding:"required"`
	Discount        float64       `json:"discount"`
	NeedsDelivery   bool          `json:"needsDelivery"`
	DeliveryAddress string        `json:"deliveryAddress"`
	Comment         string        `json:"comment"`
	Status          string        `json:"status" binding:"required"`
	Orders          []doorRequest `json:"orders" binding:"required"`
}

type orderStatusRequest struct {
	Status int `json:"status" binding:"required"`
}

func NewOrderHandler() *OrderHandler {
	return &OrderHandler{}
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req orderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректное тело запроса"})
		return
	}

	if len(req.Orders) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "заказ должен содержать хотя бы одну дверь"})
		return
	}

	if req.NeedsDelivery && strings.TrimSpace(req.DeliveryAddress) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "РЅСѓР¶РЅРѕ СѓРєР°Р·Р°С‚СЊ Р°РґСЂРµСЃ РґРѕСЃС‚Р°РІРєРё"})
		return
	}

	price := calculateOrderPrice(req.Orders)
	order := models.Order{
		Customer:        req.Customer,
		Phone:           req.Phone,
		Date:            req.Date,
		Price:           price,
		Prepayment:      req.Prepayment,
		Discount:        req.Discount,
		NeedsDelivery:   req.NeedsDelivery,
		DeliveryAddress: normalizeDeliveryAddress(req.NeedsDelivery, req.DeliveryAddress),
		Comment:         req.Comment,
		Status:          req.Status,
		InteriorDoors:   mapDoorsForCreate(req.Orders),
	}

	if err := database.DB.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось создать заказ"})
		return
	}

	if err := database.DB.Preload("InteriorDoors").First(&order, order.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось получить созданный заказ"})
		return
	}

	c.JSON(http.StatusCreated, order)
}

func (h *OrderHandler) GetOrders(c *gin.Context) {
	var orders []models.Order
	if err := database.DB.Order("id DESC").Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось получить список заказов"})
		return
	}

	c.JSON(http.StatusOK, orders)
}

func (h *OrderHandler) GetOrderByID(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var order models.Order
	if err := database.DB.Preload("InteriorDoors").First(&order, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "заказ не найден"})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) UpdateOrder(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req orderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректное тело запроса"})
		return
	}

	if len(req.Orders) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "заказ должен содержать хотя бы одну дверь"})
		return
	}

	if req.NeedsDelivery && strings.TrimSpace(req.DeliveryAddress) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "РЅСѓР¶РЅРѕ СѓРєР°Р·Р°С‚СЊ Р°РґСЂРµСЃ РґРѕСЃС‚Р°РІРєРё"})
		return
	}

	price := calculateOrderPrice(req.Orders)
	var order models.Order
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&order, id).Error; err != nil {
			return err
		}

		order.Customer = req.Customer
		order.Phone = req.Phone
		order.Date = req.Date
		order.Price = price
		order.Prepayment = req.Prepayment
		order.Discount = req.Discount
		order.NeedsDelivery = req.NeedsDelivery
		order.DeliveryAddress = normalizeDeliveryAddress(req.NeedsDelivery, req.DeliveryAddress)
		order.Comment = req.Comment
		order.Status = req.Status
		if err := tx.Save(&order).Error; err != nil {
			return err
		}

		if err := tx.Where("order_id = ?", order.ID).Delete(&models.InteriorDoor{}).Error; err != nil {
			return err
		}

		doors := mapDoorsForCreate(req.Orders)
		for i := range doors {
			doors[i].OrderID = order.ID
		}
		if err := tx.Create(&doors).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "заказ не найден"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось обновить заказ"})
		return
	}

	if err := database.DB.Preload("InteriorDoors").First(&order, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "заказ не найден"})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) DeleteOrder(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	result := database.DB.Delete(&models.Order{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось удалить заказ"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "заказ не найден"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "заказ удалён"})
}

func (h *OrderHandler) UpdateOrderStatus(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}

	var req orderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректное тело запроса"})
		return
	}

	status, statusOk := statusCodeToValue(req.Status)
	if !statusOk {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный статус"})
		return
	}

	var order models.Order
	if err := database.DB.First(&order, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "заказ не найден"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось обновить статус заказа"})
		return
	}

	if err := database.DB.Model(&order).Update("status", status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось обновить статус заказа"})
		return
	}

	if err := database.DB.Preload("InteriorDoors").First(&order, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "заказ не найден"})
		return
	}

	c.JSON(http.StatusOK, order)
}

func parseID(c *gin.Context) (uint, bool) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный id"})
		return 0, false
	}
	return uint(id), true
}

func calculateOrderPrice(doors []doorRequest) float64 {
	totalPrice := 0.0
	for _, door := range doors {
		totalPrice += door.Price * float64(door.Count)
	}
	return totalPrice
}

func statusCodeToValue(status int) (string, bool) {
	switch status {
	case 1:
		return "accepted", true
	case 2:
		return "progress", true
	case 3:
		return "completed", true
	default:
		return "", false
	}
}

func mapDoorsForCreate(doors []doorRequest) []models.InteriorDoor {
	result := make([]models.InteriorDoor, 0, len(doors))
	for _, door := range doors {
		result = append(result, models.InteriorDoor{
			Model:    door.Model,
			Price:    door.Price,
			Width:    door.Width,
			Width2:   door.Width2,
			Height:   door.Height,
			HasGlass: door.HasGlass,
			LeafType: door.LeafType,
			Count:    door.Count,
			Comment:  strings.TrimSpace(door.Comment),
		})
	}
	return result
}

func normalizeDeliveryAddress(needsDelivery bool, address string) string {
	if !needsDelivery {
		return ""
	}
	return strings.TrimSpace(address)
}
