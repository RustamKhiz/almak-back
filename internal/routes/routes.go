package routes

import (
	"time"

	"almak-back/internal/config"
	"almak-back/internal/handlers"
	"almak-back/internal/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter(cfg config.Config) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.FrontendOrigins,
		AllowWildcard:    true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	authHandler := handlers.NewAuthHandler(cfg)
	orderHandler := handlers.NewOrderHandler()
	registerRoutes(router.Group("/api"), cfg, authHandler, orderHandler)
	registerRoutes(router.Group("/"), cfg, authHandler, orderHandler)

	return router
}

func registerRoutes(
	group *gin.RouterGroup,
	cfg config.Config,
	authHandler *handlers.AuthHandler,
	orderHandler *handlers.OrderHandler,
) {
	group.POST("/login", authHandler.Login)

	protected := group.Group("/")
	protected.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	{
		protected.POST("/orders", orderHandler.CreateOrder)
		protected.GET("/orders", orderHandler.GetOrders)
		protected.GET("/orders/:id", orderHandler.GetOrderByID)
		protected.PUT("/orders/:id", orderHandler.UpdateOrder)
		protected.PATCH("/orders/:id/status", orderHandler.UpdateOrderStatus)
		protected.PATCH("/orders/:id/payment-status", orderHandler.UpdateOrderPaymentStatus)
		protected.POST("/orders/:id/payments", orderHandler.AddOrderPayment)
		protected.POST("/orders/:id/payments/:paymentId/reverse", orderHandler.ReverseOrderPayment)
		protected.POST("/orders/:id/discounts", orderHandler.AddOrderDiscount)
		protected.DELETE("/orders/:id", orderHandler.DeleteOrder)
	}
}
