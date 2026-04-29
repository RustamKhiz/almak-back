package handlers

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"almak-back/internal/database"
	"almak-back/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type OrderHandler struct{}

type interiorDoorRequest struct {
	Model        string   `json:"model" binding:"required"`
	Color        string   `json:"color" binding:"required"`
	Price        float64  `json:"price" binding:"required"`
	Price2       *float64 `json:"price2"`
	Width        int      `json:"width" binding:"required"`
	Width2       *int     `json:"width2"`
	Height       int      `json:"height" binding:"required"`
	Height2      *int     `json:"height2"`
	HasGlass     bool     `json:"hasGlass"`
	GlassComment string   `json:"glassComment"`
	LeafType     string   `json:"leafType" binding:"required"`
	Count        int      `json:"count" binding:"required"`
	Count2       *int     `json:"count2"`
	Covering     string   `json:"covering" binding:"required"`
	Comment      string   `json:"comment"`
}
type entranceDoorRequest struct {
	Kind        string  `json:"kind" binding:"required"`
	LeafType    string  `json:"leafType" binding:"required"`
	Model       string  `json:"model" binding:"required"`
	Width       int     `json:"width" binding:"required"`
	Height      int     `json:"height" binding:"required"`
	Color       string  `json:"color" binding:"required"`
	Painting    *string `json:"painting"`
	PanelColor  *string `json:"panelColor"`
	HasPeephole *bool   `json:"hasPeephole"`
	Count       int     `json:"count" binding:"required"`
	Price       float64 `json:"price" binding:"required"`
	Comment     string  `json:"comment"`
}
type moldingRequest struct {
	FrameLength    *int     `json:"frameLength"`
	FramePrice     *float64 `json:"framePrice"`
	FrameCount     float64  `json:"frameCount" binding:"required"`
	PlatbandType   string   `json:"platbandType" binding:"required"`
	PlatbandFigure *string  `json:"platbandFigure"`
	PlatbandLength *int     `json:"platbandLength"`
	PlatbandPrice  float64  `json:"platbandPrice" binding:"required"`
	PlatbandCount  float64  `json:"platbandCount" binding:"required"`
	RebateBarCount int      `json:"rebateBarCount"`
	Color          string   `json:"color" binding:"required"`
	Covering       string   `json:"covering" binding:"required"`
	Comment        string   `json:"comment"`
}
type extensionRequest struct {
	Color          string  `json:"color" binding:"required"`
	Covering       string  `json:"covering" binding:"required"`
	Width          int     `json:"width" binding:"required"`
	Height         int     `json:"height" binding:"required"`
	QuantityPerSet float64 `json:"quantityPerSet" binding:"required"`
	TotalArea      float64 `json:"totalArea" binding:"required"`
	Comment        string  `json:"comment"`
	Count          float64 `json:"count" binding:"required"`
	Price          float64 `json:"price" binding:"required"`
}
type capitalRequest struct {
	Name     string  `json:"name" binding:"required"`
	Color    string  `json:"color" binding:"required"`
	Covering string  `json:"covering" binding:"required"`
	Width    int     `json:"width" binding:"required"`
	Height   int     `json:"height" binding:"required"`
	Price    float64 `json:"price" binding:"required"`
	Comment  string  `json:"comment"`
	Count    int     `json:"count" binding:"required"`
}
type hardwareRequest struct {
	HandleModel     *string  `json:"handleModel"`
	HandleColor     *string  `json:"handleColor"`
	HandleCount     *int     `json:"handleCount"`
	HandlePrice     *float64 `json:"handlePrice"`
	LockCount       *int     `json:"lockCount"`
	LockPrice       *float64 `json:"lockPrice"`
	FixatorCount    *int     `json:"fixatorCount"`
	FixatorPrice    *float64 `json:"fixatorPrice"`
	ThumbturnCount  *int     `json:"thumbturnCount"`
	ThumbturnPrice  *float64 `json:"thumbturnPrice"`
	EscutcheonCount *int     `json:"escutcheonCount"`
	EscutcheonPrice *float64 `json:"escutcheonPrice"`
	CylinderCount   *int     `json:"cylinderCount"`
	CylinderPrice   *float64 `json:"cylinderPrice"`
	BoltCount       *int     `json:"boltCount"`
	BoltPrice       *float64 `json:"boltPrice"`
	HingeCount      *int     `json:"hingeCount"`
	HingePrice      *float64 `json:"hingePrice"`
	DoorStopCount   *int     `json:"doorStopCount"`
	DoorStopPrice   *float64 `json:"doorStopPrice"`
	Comment         string   `json:"comment"`
}
type panelingRequest struct {
	Color          string  `json:"color" binding:"required"`
	Width          int     `json:"width" binding:"required"`
	Height         int     `json:"height" binding:"required"`
	Covering       string  `json:"covering" binding:"required"`
	QuantityPerSet float64 `json:"quantityPerSet" binding:"required"`
	TotalArea      float64 `json:"totalArea" binding:"required"`
	Count          int     `json:"count" binding:"required"`
	Price          float64 `json:"price" binding:"required"`
	Comment        string  `json:"comment"`
}

type orderRequest struct {
	Customer        string                `json:"customer" binding:"required"`
	Phone           string                `json:"phone" binding:"required"`
	Date            string                `json:"date" binding:"required"`
	Prepayment      float64               `json:"prepayment" binding:"required"`
	Discount        float64               `json:"discount"`
	NeedsDelivery   bool                  `json:"needsDelivery"`
	DeliveryAddress string                `json:"deliveryAddress"`
	Comment         string                `json:"comment"`
	Status          string                `json:"status" binding:"required"`
	IsPaid          bool                  `json:"isPaid"`
	InteriorDoors   []interiorDoorRequest `json:"interiorDoors"`
	EntranceDoors   []entranceDoorRequest `json:"entranceDoors"`
	Moldings        []moldingRequest      `json:"moldings"`
	Extensions      []extensionRequest    `json:"extensions"`
	Capitals        []capitalRequest      `json:"capitals"`
	Hardwares       []hardwareRequest     `json:"hardwares"`
	Panelings       []panelingRequest     `json:"panelings"`
}

type orderStatusRequest struct {
	Status int `json:"status" binding:"required"`
}

type orderPaymentStatusRequest struct {
	IsPaid bool `json:"isPaid"`
}

type addOrderPaymentRequest struct {
	Amount  float64 `json:"amount" binding:"required"`
	Comment string  `json:"comment"`
}

func NewOrderHandler() *OrderHandler { return &OrderHandler{} }

func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req orderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if !hasOrderItems(req) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order must contain at least one item"})
		return
	}
	if req.NeedsDelivery && strings.TrimSpace(req.DeliveryAddress) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "delivery address is required"})
		return
	}
	if req.Prepayment <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepayment must be greater than zero"})
		return
	}
	if hasInteriorDoorGlassWithoutComment(req.InteriorDoors) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "glass comment is required for glass interior doors"})
		return
	}
	order := models.Order{Customer: req.Customer, Phone: req.Phone, Date: req.Date, Price: calculateOrderPrice(req), Prepayment: 0, Discount: req.Discount, NeedsDelivery: req.NeedsDelivery, DeliveryAddress: normalizeDeliveryAddress(req.NeedsDelivery, req.DeliveryAddress), Comment: req.Comment, Status: req.Status, IsPaid: req.IsPaid, InteriorDoors: mapInteriorDoorsForCreate(req.InteriorDoors), EntranceDoors: mapEntranceDoorsForCreate(req.EntranceDoors), Moldings: mapMoldingsForCreate(req.Moldings), Extensions: mapExtensionsForCreate(req.Extensions), Capitals: mapCapitalsForCreate(req.Capitals), Hardwares: mapHardwaresForCreate(req.Hardwares), Panelings: mapPanelingsForCreate(req.Panelings)}
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		if req.Prepayment > 0 {
			if err := createOrderPayment(tx, order.ID, req.Prepayment, "Первоначальный взнос"); err != nil {
				return err
			}
		}
		return syncOrderPrepayment(tx, order.ID)
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order"})
		return
	}
	if err := preloadOrder(database.DB).First(&order, order.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load created order"})
		return
	}
	c.JSON(http.StatusCreated, order)
}

func (h *OrderHandler) GetOrders(c *gin.Context) {
	var orders []models.Order
	if err := database.DB.Order("id DESC").Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load orders"})
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
	if err := preloadOrder(database.DB).First(&order, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if !hasOrderItems(req) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order must contain at least one item"})
		return
	}
	if req.NeedsDelivery && strings.TrimSpace(req.DeliveryAddress) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "delivery address is required"})
		return
	}
	if req.Prepayment <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepayment must be greater than zero"})
		return
	}
	if hasInteriorDoorGlassWithoutComment(req.InteriorDoors) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "glass comment is required for glass interior doors"})
		return
	}
	var order models.Order
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&order, id).Error; err != nil {
			return err
		}
		currentPrepayment, err := getOrderPaidAmount(tx, order.ID)
		if err != nil {
			return err
		}
		order.Customer = req.Customer
		order.Phone = req.Phone
		order.Date = req.Date
		order.Price = calculateOrderPrice(req)
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
		if err := tx.Where("order_id = ?", order.ID).Delete(&models.EntranceDoor{}).Error; err != nil {
			return err
		}
		if err := tx.Where("order_id = ?", order.ID).Delete(&models.Molding{}).Error; err != nil {
			return err
		}
		if err := tx.Where("order_id = ?", order.ID).Delete(&models.Extension{}).Error; err != nil {
			return err
		}
		if err := tx.Where("order_id = ?", order.ID).Delete(&models.Capital{}).Error; err != nil {
			return err
		}
		if err := tx.Where("order_id = ?", order.ID).Delete(&models.Hardware{}).Error; err != nil {
			return err
		}
		if err := tx.Where("order_id = ?", order.ID).Delete(&models.Paneling{}).Error; err != nil {
			return err
		}
		interiorDoors := mapInteriorDoorsForCreate(req.InteriorDoors)
		for i := range interiorDoors {
			interiorDoors[i].OrderID = order.ID
		}
		if len(interiorDoors) > 0 {
			if err := tx.Create(&interiorDoors).Error; err != nil {
				return err
			}
		}
		entranceDoors := mapEntranceDoorsForCreate(req.EntranceDoors)
		for i := range entranceDoors {
			entranceDoors[i].OrderID = order.ID
		}
		if len(entranceDoors) > 0 {
			if err := tx.Create(&entranceDoors).Error; err != nil {
				return err
			}
		}
		moldings := mapMoldingsForCreate(req.Moldings)
		for i := range moldings {
			moldings[i].OrderID = order.ID
		}
		if len(moldings) > 0 {
			if err := tx.Create(&moldings).Error; err != nil {
				return err
			}
		}
		extensions := mapExtensionsForCreate(req.Extensions)
		for i := range extensions {
			extensions[i].OrderID = order.ID
		}
		if len(extensions) > 0 {
			if err := tx.Create(&extensions).Error; err != nil {
				return err
			}
		}
		capitals := mapCapitalsForCreate(req.Capitals)
		for i := range capitals {
			capitals[i].OrderID = order.ID
		}
		if len(capitals) > 0 {
			if err := tx.Create(&capitals).Error; err != nil {
				return err
			}
		}
		hardwares := mapHardwaresForCreate(req.Hardwares)
		for i := range hardwares {
			hardwares[i].OrderID = order.ID
		}
		if len(hardwares) > 0 {
			if err := tx.Create(&hardwares).Error; err != nil {
				return err
			}
		}
		panelings := mapPanelingsForCreate(req.Panelings)
		for i := range panelings {
			panelings[i].OrderID = order.ID
		}
		if len(panelings) > 0 {
			if err := tx.Create(&panelings).Error; err != nil {
				return err
			}
		}
		delta := roundMoney(req.Prepayment - currentPrepayment)
		if delta != 0 {
			if err := createOrderPayment(tx, order.ID, delta, "Корректировка внесенной суммы из редактирования заказа"); err != nil {
				return err
			}
		}
		return syncOrderPrepayment(tx, order.ID)
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update order"})
		return
	}
	if err := preloadOrder(database.DB).First(&order, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete order"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "order deleted"})
}
func (h *OrderHandler) UpdateOrderStatus(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req orderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	status, statusOk := statusCodeToValue(req.Status)
	if !statusOk {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	var order models.Order
	if err := database.DB.First(&order, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update order status"})
		return
	}
	if err := database.DB.Model(&order).Update("status", status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update order status"})
		return
	}
	if err := preloadOrder(database.DB).First(&order, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) UpdateOrderPaymentStatus(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req orderPaymentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	var order models.Order
	if err := database.DB.First(&order, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update order payment status"})
		return
	}
	if err := database.DB.Model(&order).Update("is_paid", req.IsPaid).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update order payment status"})
		return
	}
	if err := preloadOrder(database.DB).First(&order, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) AddOrderPayment(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req addOrderPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payment amount must be greater than zero"})
		return
	}

	var order models.Order
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&order, id).Error; err != nil {
			return err
		}
		if err := createOrderPayment(tx, order.ID, req.Amount, req.Comment); err != nil {
			return err
		}
		return syncOrderPrepayment(tx, order.ID)
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add order payment"})
		return
	}
	if err := preloadOrder(database.DB).First(&order, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) ReverseOrderPayment(c *gin.Context) {
	orderID, ok := parseID(c)
	if !ok {
		return
	}
	paymentID, err := strconv.ParseUint(c.Param("paymentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment id"})
		return
	}

	var order models.Order
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&order, orderID).Error; err != nil {
			return err
		}

		var payment models.OrderPayment
		if err := tx.Where("order_id = ?", orderID).First(&payment, uint(paymentID)).Error; err != nil {
			return err
		}
		if payment.ReversalOfPaymentID != nil {
			return errors.New("payment reversal cannot be reversed")
		}
		if payment.ReversedByPaymentID != nil {
			return errors.New("payment already reversed")
		}

		comment := "Сторно платежа"
		if strings.TrimSpace(payment.Comment) != "" {
			comment += ": " + strings.TrimSpace(payment.Comment)
		}
		reversal := models.OrderPayment{
			OrderID:             order.ID,
			Amount:              roundMoney(-payment.Amount),
			Comment:             comment,
			ReversalOfPaymentID: &payment.ID,
		}
		if err := tx.Create(&reversal).Error; err != nil {
			return err
		}
		if err := tx.Model(&payment).Update("reversed_by_payment_id", reversal.ID).Error; err != nil {
			return err
		}
		return syncOrderPrepayment(tx, order.ID)
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order or payment not found"})
			return
		}
		switch err.Error() {
		case "payment reversal cannot be reversed", "payment already reversed":
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reverse order payment"})
			return
		}
	}
	if err := preloadOrder(database.DB).First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return uint(id), true
}
func calculateOrderPrice(req orderRequest) float64 {
	total := 0.0
	for _, door := range req.InteriorDoors {
		total += calculateInteriorDoorPrice(door)
	}
	for _, door := range req.EntranceDoors {
		total += door.Price * float64(door.Count)
	}
	for _, item := range req.Moldings {
		total += derefFloat64OrZero(item.FramePrice)*item.FrameCount + item.PlatbandPrice*item.PlatbandCount
	}
	for _, item := range req.Extensions {
		total += item.TotalArea * item.Price * item.Count
	}
	for _, item := range req.Capitals {
		total += item.Price * float64(item.Count)
	}
	for _, item := range req.Hardwares {
		total += calculateHardwarePrice(item)
	}
	for _, item := range req.Panelings {
		total += item.TotalArea * item.Price * float64(item.Count)
	}
	return total
}
func statusCodeToValue(status int) (string, bool) {
	switch status {
	case 1:
		return "accepted", true
	case 2:
		return "ordered", true
	case 3:
		return "received", true
	case 4:
		return "customer_notified", true
	case 5:
		return "issued", true
	case 6:
		return "completed", true
	default:
		return "", false
	}
}
func mapInteriorDoorsForCreate(doors []interiorDoorRequest) []models.InteriorDoor {
	result := make([]models.InteriorDoor, 0, len(doors))
	for _, door := range doors {
		leafType := normalizeDoorLeafType(door.LeafType)
		result = append(result, models.InteriorDoor{Model: strings.TrimSpace(door.Model), Color: strings.TrimSpace(door.Color), Price: door.Price, Price2: normalizeSecondLeafFloat64(leafType, door.Price2), Width: door.Width, Width2: normalizeSecondLeafInt(leafType, door.Width2), Height: door.Height, Height2: normalizeSecondLeafInt(leafType, door.Height2), HasGlass: door.HasGlass, GlassComment: normalizeGlassComment(door.HasGlass, door.GlassComment), LeafType: leafType, Count: door.Count, Count2: normalizeSecondLeafInt(leafType, door.Count2), Covering: door.Covering, Comment: strings.TrimSpace(door.Comment)})
	}
	return result
}
func mapEntranceDoorsForCreate(doors []entranceDoorRequest) []models.EntranceDoor {
	result := make([]models.EntranceDoor, 0, len(doors))
	for _, door := range doors {
		result = append(result, models.EntranceDoor{Kind: strings.TrimSpace(door.Kind), LeafType: normalizeDoorLeafType(door.LeafType), Model: strings.TrimSpace(door.Model), Width: door.Width, Height: door.Height, Color: strings.TrimSpace(door.Color), Painting: normalizeOptionalString(door.Painting), PanelColor: normalizeOptionalString(door.PanelColor), HasPeephole: door.HasPeephole, Count: door.Count, Price: door.Price, Comment: strings.TrimSpace(door.Comment)})
	}
	return result
}
func mapMoldingsForCreate(items []moldingRequest) []models.Molding {
	result := make([]models.Molding, 0, len(items))
	for _, item := range items {
		result = append(result, models.Molding{FrameLength: normalizeOptionalInt(item.FrameLength), FramePrice: derefFloat64OrZero(item.FramePrice), FrameCount: item.FrameCount, PlatbandType: strings.TrimSpace(item.PlatbandType), PlatbandFigure: normalizeOptionalString(item.PlatbandFigure), PlatbandLength: normalizeOptionalInt(item.PlatbandLength), PlatbandPrice: item.PlatbandPrice, PlatbandCount: item.PlatbandCount, RebateBarCount: item.RebateBarCount, Color: strings.TrimSpace(item.Color), Covering: strings.TrimSpace(item.Covering), Comment: strings.TrimSpace(item.Comment)})
	}
	return result
}
func mapExtensionsForCreate(items []extensionRequest) []models.Extension {
	result := make([]models.Extension, 0, len(items))
	for _, item := range items {
		result = append(result, models.Extension{Color: strings.TrimSpace(item.Color), Covering: strings.TrimSpace(item.Covering), Width: item.Width, Height: item.Height, QuantityPerSet: normalizeExtensionQuantityPerSet(item.QuantityPerSet), TotalArea: normalizeExtensionTotalArea(item.Width, item.Height, item.QuantityPerSet, item.TotalArea), Comment: strings.TrimSpace(item.Comment), Count: item.Count, Price: item.Price})
	}
	return result
}
func mapCapitalsForCreate(items []capitalRequest) []models.Capital {
	result := make([]models.Capital, 0, len(items))
	for _, item := range items {
		result = append(result, models.Capital{Name: strings.TrimSpace(item.Name), Color: strings.TrimSpace(item.Color), Covering: strings.TrimSpace(item.Covering), Width: item.Width, Height: item.Height, Price: item.Price, Comment: strings.TrimSpace(item.Comment), Count: item.Count})
	}
	return result
}
func mapHardwaresForCreate(items []hardwareRequest) []models.Hardware {
	result := make([]models.Hardware, 0, len(items))
	for _, item := range items {
		if isHardwareEmpty(item) {
			continue
		}
		result = append(result, models.Hardware{HandleModel: normalizeOptionalString(item.HandleModel), HandleColor: normalizeOptionalString(item.HandleColor), HandleCount: normalizeOptionalInt(item.HandleCount), HandlePrice: normalizeOptionalFloat64(item.HandlePrice), LockCount: normalizeOptionalInt(item.LockCount), LockPrice: normalizeOptionalFloat64(item.LockPrice), FixatorCount: normalizeOptionalInt(item.FixatorCount), FixatorPrice: normalizeOptionalFloat64(item.FixatorPrice), ThumbturnCount: normalizeOptionalInt(item.ThumbturnCount), ThumbturnPrice: normalizeOptionalFloat64(item.ThumbturnPrice), EscutcheonCount: normalizeOptionalInt(item.EscutcheonCount), EscutcheonPrice: normalizeOptionalFloat64(item.EscutcheonPrice), CylinderCount: normalizeOptionalInt(item.CylinderCount), CylinderPrice: normalizeOptionalFloat64(item.CylinderPrice), BoltCount: normalizeOptionalInt(item.BoltCount), BoltPrice: normalizeOptionalFloat64(item.BoltPrice), HingeCount: normalizeOptionalInt(item.HingeCount), HingePrice: normalizeOptionalFloat64(item.HingePrice), DoorStopCount: normalizeOptionalInt(item.DoorStopCount), DoorStopPrice: normalizeOptionalFloat64(item.DoorStopPrice), Comment: strings.TrimSpace(item.Comment)})
	}
	return result
}
func mapPanelingsForCreate(items []panelingRequest) []models.Paneling {
	result := make([]models.Paneling, 0, len(items))
	for _, item := range items {
		result = append(result, models.Paneling{Color: strings.TrimSpace(item.Color), Size: formatSize(item.Width, item.Height), Width: item.Width, Height: item.Height, Covering: strings.TrimSpace(item.Covering), QuantityPerSet: normalizeExtensionQuantityPerSet(item.QuantityPerSet), TotalArea: normalizeExtensionTotalArea(item.Width, item.Height, item.QuantityPerSet, item.TotalArea), Count: item.Count, Price: item.Price, Comment: strings.TrimSpace(item.Comment)})
	}
	return result
}
func normalizeDeliveryAddress(needsDelivery bool, address string) string {
	if !needsDelivery {
		return ""
	}
	return strings.TrimSpace(address)
}
func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil
	}
	return &normalized
}
func normalizeOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}
	normalized := *value
	if normalized <= 0 {
		return nil
	}
	return &normalized
}
func normalizeOptionalFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	normalized := *value
	if normalized <= 0 {
		return nil
	}
	return &normalized
}

func normalizeSecondLeafInt(leafType string, value *int) *int {
	if leafType != "Double" || value == nil || *value <= 0 {
		return nil
	}

	normalized := *value
	return &normalized
}

func normalizeSecondLeafFloat64(leafType string, value *float64) *float64 {
	if leafType != "Double" || value == nil || *value < 0 {
		return nil
	}

	normalized := *value
	return &normalized
}

func derefFloat64OrZero(value *float64) float64 {
	if value == nil {
		return 0
	}

	return *value
}

func normalizeGlassComment(hasGlass bool, value string) string {
	if !hasGlass {
		return ""
	}

	return strings.TrimSpace(value)
}

func hasInteriorDoorGlassWithoutComment(doors []interiorDoorRequest) bool {
	for _, door := range doors {
		if door.HasGlass && strings.TrimSpace(door.GlassComment) == "" {
			return true
		}
	}

	return false
}

func normalizeDoorLeafType(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "Double") {
		return "Double"
	}

	return "Single"
}

func normalizeExtensionQuantityPerSet(value float64) float64 {
	if value <= 0 {
		return 0.5
	}

	return value
}

func normalizeExtensionTotalArea(width int, height int, quantityPerSet float64, totalArea float64) float64 {
	if totalArea > 0 {
		return totalArea
	}

	return float64(width) * float64(height) * normalizeExtensionQuantityPerSet(quantityPerSet) / 10000
}

func formatSize(width int, height int) string {
	return strconv.Itoa(width) + "x" + strconv.Itoa(height)
}

func hasOrderItems(req orderRequest) bool {
	return len(req.InteriorDoors) > 0 || len(req.EntranceDoors) > 0 || len(req.Moldings) > 0 || len(req.Extensions) > 0 || len(req.Capitals) > 0 || len(mapHardwaresForCreate(req.Hardwares)) > 0 || len(req.Panelings) > 0
}
func preloadOrder(db *gorm.DB) *gorm.DB {
	return db.Preload("Payments", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("created_at ASC, id ASC")
	}).Preload("InteriorDoors").Preload("EntranceDoors").Preload("Moldings").Preload("Extensions").Preload("Capitals").Preload("Hardwares").Preload("Panelings")
}
func calculateHardwarePrice(item hardwareRequest) float64 {
	return optionalLineTotal(item.HandleCount, item.HandlePrice) + optionalLineTotal(item.LockCount, item.LockPrice) + optionalLineTotal(item.FixatorCount, item.FixatorPrice) + optionalLineTotal(item.ThumbturnCount, item.ThumbturnPrice) + optionalLineTotal(item.EscutcheonCount, item.EscutcheonPrice) + optionalLineTotal(item.CylinderCount, item.CylinderPrice) + optionalLineTotal(item.BoltCount, item.BoltPrice) + optionalLineTotal(item.HingeCount, item.HingePrice) + optionalLineTotal(item.DoorStopCount, item.DoorStopPrice)
}

func calculateInteriorDoorPrice(item interiorDoorRequest) float64 {
	total := item.Price * float64(item.Count)
	if normalizeDoorLeafType(item.LeafType) == "Double" && item.Price2 != nil && item.Count2 != nil {
		total += *item.Price2 * float64(*item.Count2)
	}
	return total
}

func optionalLineTotal(count *int, price *float64) float64 {
	if count == nil || price == nil {
		return 0
	}
	return float64(*count) * *price
}
func isHardwareEmpty(item hardwareRequest) bool {
	return normalizeOptionalString(item.HandleModel) == nil && normalizeOptionalString(item.HandleColor) == nil && normalizeOptionalInt(item.HandleCount) == nil && normalizeOptionalFloat64(item.HandlePrice) == nil && normalizeOptionalInt(item.LockCount) == nil && normalizeOptionalFloat64(item.LockPrice) == nil && normalizeOptionalInt(item.FixatorCount) == nil && normalizeOptionalFloat64(item.FixatorPrice) == nil && normalizeOptionalInt(item.ThumbturnCount) == nil && normalizeOptionalFloat64(item.ThumbturnPrice) == nil && normalizeOptionalInt(item.EscutcheonCount) == nil && normalizeOptionalFloat64(item.EscutcheonPrice) == nil && normalizeOptionalInt(item.CylinderCount) == nil && normalizeOptionalFloat64(item.CylinderPrice) == nil && normalizeOptionalInt(item.BoltCount) == nil && normalizeOptionalFloat64(item.BoltPrice) == nil && normalizeOptionalInt(item.HingeCount) == nil && normalizeOptionalFloat64(item.HingePrice) == nil && normalizeOptionalInt(item.DoorStopCount) == nil && normalizeOptionalFloat64(item.DoorStopPrice) == nil && strings.TrimSpace(item.Comment) == ""
}

func createOrderPayment(tx *gorm.DB, orderID uint, amount float64, comment string) error {
	payment := models.OrderPayment{
		OrderID: orderID,
		Amount:  roundMoney(amount),
		Comment: strings.TrimSpace(comment),
	}
	return tx.Create(&payment).Error
}

func getOrderPaidAmount(tx *gorm.DB, orderID uint) (float64, error) {
	var total float64
	if err := tx.Model(&models.OrderPayment{}).
		Where("order_id = ?", orderID).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error; err != nil {
		return 0, err
	}
	return roundMoney(total), nil
}

func syncOrderPrepayment(tx *gorm.DB, orderID uint) error {
	total, err := getOrderPaidAmount(tx, orderID)
	if err != nil {
		return err
	}

	var order models.Order
	if err := tx.Select("id", "price", "discount").First(&order, orderID).Error; err != nil {
		return err
	}

	totalToPay := math.Max(roundMoney(order.Price-order.Discount), 0)
	isPaid := roundMoney(total) >= totalToPay
	return tx.Model(&models.Order{}).Where("id = ?", orderID).Updates(map[string]any{
		"prepayment": total,
		"is_paid":    isPaid,
	}).Error
}

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}
