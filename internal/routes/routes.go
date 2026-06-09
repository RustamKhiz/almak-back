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
	catalogHandler := handlers.NewCatalogHandler()
	registerRoutes(router.Group("/api"), cfg, authHandler, orderHandler, catalogHandler)
	registerRoutes(router.Group("/"), cfg, authHandler, orderHandler, catalogHandler)

	return router
}

func registerRoutes(
	group *gin.RouterGroup,
	cfg config.Config,
	authHandler *handlers.AuthHandler,
	orderHandler *handlers.OrderHandler,
	catalogHandler *handlers.CatalogHandler,
) {
	group.POST("/login", authHandler.Login)
	group.POST("/refresh", authHandler.Refresh)

	protected := group.Group("/")
	protected.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	{
		protected.POST("/orders", orderHandler.CreateOrder)
		protected.GET("/orders", orderHandler.GetOrders)
		protected.GET("/orders/:id", orderHandler.GetOrderByID)
		protected.PUT("/orders/:id", orderHandler.UpdateOrder)
		protected.PATCH("/orders/:id/status", orderHandler.UpdateOrderStatus)
		protected.POST("/orders/:id/payments", orderHandler.AddOrderPayment)
		protected.POST("/orders/:id/payments/:paymentId/reverse", orderHandler.ReverseOrderPayment)
		protected.PATCH("/orders/:id/discounts", orderHandler.UpdateOrderDiscount)
		protected.DELETE("/orders/:id", orderHandler.DeleteOrder)

		protected.GET("/catalogs", catalogHandler.GetCatalogs)
		protected.POST("/catalogs", catalogHandler.CreateCatalog)
		protected.PUT("/catalogs/:id", catalogHandler.UpdateCatalog)
		protected.DELETE("/catalogs/:id", catalogHandler.DeleteCatalog)
		protected.GET("/catalogs/key/:key/items", catalogHandler.GetCatalogItemsByKey)
		protected.GET("/catalogs/:id/items", catalogHandler.GetCatalogItems)
		protected.POST("/catalogs/:id/items", catalogHandler.CreateCatalogItem)
		protected.PUT("/catalogs/:id/items/:itemId", catalogHandler.UpdateCatalogItem)
		protected.DELETE("/catalogs/:id/items/:itemId", catalogHandler.DeleteCatalogItem)
	}
}
