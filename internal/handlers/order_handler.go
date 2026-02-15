package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"almak-back/internal/database"
	"almak-back/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type OrderHandler struct{}

type doorRequest struct {
	Type     string  `json:"type" binding:"required"`
	Model    string  `json:"model" binding:"required"`
	Price    float64 `json:"price" binding:"required"`
	Color    string  `json:"color" binding:"required"`
	Width    int     `json:"width" binding:"required"`
	Height   int     `json:"height" binding:"required"`
	LeafType string  `json:"leafType" binding:"required"`
	Count    int     `json:"count" binding:"required"`
}

type orderRequest struct {
	Customer   string        `json:"customer" binding:"required"`
	Phone      string        `json:"phone" binding:"required"`
	Date       string        `json:"date" binding:"required"`
	Prepayment float64       `json:"prepayment" binding:"required"`
	Comment    string        `json:"comment"`
	Status     string        `json:"status" binding:"required"`
	Orders     []doorRequest `json:"orders" binding:"required"`
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

	count, price := aggregateDoors(req.Orders)
	order := models.Order{
		Customer:   req.Customer,
		Phone:      req.Phone,
		Date:       req.Date,
		Count:      count,
		Price:      price,
		Prepayment: req.Prepayment,
		Comment:    req.Comment,
		Status:     req.Status,
		Doors:      mapDoorsForCreate(req.Orders),
	}

	if err := database.DB.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось создать заказ"})
		return
	}

	if err := database.DB.Preload("Doors").First(&order, order.ID).Error; err != nil {
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
	if err := database.DB.Preload("Doors").First(&order, id).Error; err != nil {
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

	count, price := aggregateDoors(req.Orders)
	var order models.Order
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&order, id).Error; err != nil {
			return err
		}

		order.Customer = req.Customer
		order.Phone = req.Phone
		order.Date = req.Date
		order.Count = count
		order.Price = price
		order.Prepayment = req.Prepayment
		order.Comment = req.Comment
		order.Status = req.Status
		if err := tx.Save(&order).Error; err != nil {
			return err
		}

		if err := tx.Where("order_id = ?", order.ID).Delete(&models.Door{}).Error; err != nil {
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

	if err := database.DB.Preload("Doors").First(&order, id).Error; err != nil {
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

func parseID(c *gin.Context) (uint, bool) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный id"})
		return 0, false
	}
	return uint(id), true
}

func aggregateDoors(doors []doorRequest) (int, float64) {
	totalCount := 0
	totalPrice := 0.0
	for _, door := range doors {
		totalCount += door.Count
		totalPrice += door.Price * float64(door.Count)
	}
	return totalCount, totalPrice
}

func mapDoorsForCreate(doors []doorRequest) []models.Door {
	result := make([]models.Door, 0, len(doors))
	for _, door := range doors {
		result = append(result, models.Door{
			Type:     door.Type,
			Model:    door.Model,
			Price:    door.Price,
			Color:    door.Color,
			Width:    door.Width,
			Height:   door.Height,
			LeafType: door.LeafType,
			Count:    door.Count,
		})
	}
	return result
}
