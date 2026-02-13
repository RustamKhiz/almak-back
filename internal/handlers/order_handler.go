package handlers

import (
	"net/http"
	"strconv"

	"almak-back/internal/database"
	"almak-back/internal/models"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct{}

type orderRequest struct {
	Customer   string  `json:"customer" binding:"required"`
	Phone      string  `json:"phone" binding:"required"`
	Date       string  `json:"date" binding:"required"`
	Count      int     `json:"count" binding:"required"`
	Price      float64 `json:"price" binding:"required"`
	Prepayment float64 `json:"prepayment" binding:"required"`
	Comment    string  `json:"comment"`
	Status     string  `json:"status" binding:"required"`
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

	order := models.Order{
		Customer:   req.Customer,
		Phone:      req.Phone,
		Date:       req.Date,
		Count:      req.Count,
		Price:      req.Price,
		Prepayment: req.Prepayment,
		Comment:    req.Comment,
		Status:     req.Status,
	}

	if err := database.DB.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось создать заказ"})
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
	if err := database.DB.First(&order, id).Error; err != nil {
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

	var order models.Order
	if err := database.DB.First(&order, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "заказ не найден"})
		return
	}

	order.Customer = req.Customer
	order.Phone = req.Phone
	order.Date = req.Date
	order.Count = req.Count
	order.Price = req.Price
	order.Prepayment = req.Prepayment
	order.Comment = req.Comment
	order.Status = req.Status

	if err := database.DB.Save(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось обновить заказ"})
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
