package routes

import (
	"time"

	"almak-back/internal/config"
	"almak-back/internal/handlers"
	"almak-back/internal/middleware"
	"almak-back/internal/models"

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
	aiHandler := handlers.NewAiOrderHandler(cfg)
	registerRoutes(router.Group("/api"), cfg, authHandler, orderHandler, catalogHandler, aiHandler)
	registerRoutes(router.Group("/"), cfg, authHandler, orderHandler, catalogHandler, aiHandler)

	return router
}

func registerRoutes(
	group *gin.RouterGroup,
	cfg config.Config,
	authHandler *handlers.AuthHandler,
	orderHandler *handlers.OrderHandler,
	catalogHandler *handlers.CatalogHandler,
	aiHandler *handlers.AiOrderHandler,
) {
	group.POST("/login", authHandler.Login)
	group.POST("/refresh", authHandler.Refresh)
	group.POST("/logout", authHandler.Logout)
	group.POST("/desktop/login", authHandler.DesktopLogin)
	group.POST("/desktop/refresh", authHandler.DesktopRefresh)

	protected := group.Group("/")
	protected.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	{
		protected.GET("/me", authHandler.Me)
		drafts := &handlers.DraftHandler{}
		protected.GET("/order-drafts", middleware.RequirePermission(models.DraftsWrite), drafts.List)
		protected.GET("/order-drafts/:id", middleware.RequirePermission(models.DraftsWrite), drafts.Get)
		protected.POST("/order-drafts", middleware.RequirePermission(models.DraftsWrite), drafts.Create)
		protected.PUT("/order-drafts/:id", middleware.RequirePermission(models.DraftsWrite), drafts.Update)
		protected.POST("/order-drafts/import", middleware.RequirePermission(models.DraftsWrite), drafts.Import)
		protected.POST("/order-drafts/:id/archive", middleware.RequirePermission(models.DraftsWrite), drafts.Archive)
		protected.POST("/order-drafts/:id/restore", middleware.RequirePermission(models.DraftsWrite), drafts.Restore)
		protected.POST("/order-drafts/:id/undo", middleware.RequirePermission(models.DraftsWrite), drafts.Undo)
		protected.POST("/order-drafts/:id/messages", middleware.RequirePermission(models.DraftsWrite), middleware.RequirePermission(models.AssistantUse), aiHandler.SendDraftMessage)

		protected.POST("/ai/order-drafts/parse", middleware.RequirePermission(models.AssistantUse), aiHandler.Parse)
		protected.POST("/orders", middleware.RequirePermission(models.OrdersWrite), orderHandler.CreateOrder)
		protected.GET("/orders", middleware.RequirePermission(models.OrdersRead), orderHandler.GetOrders)
		protected.GET("/orders/analytics/suppliers", middleware.RequirePermission(models.AnalyticsRead), orderHandler.GetSupplierStats)
		protected.GET("/orders/:id", middleware.RequirePermission(models.OrdersRead), orderHandler.GetOrderByID)
		protected.PUT("/orders/:id", middleware.RequirePermission(models.OrdersWrite), orderHandler.UpdateOrder)
		protected.PATCH("/orders/:id/status", middleware.RequirePermission(models.OrdersWrite), orderHandler.UpdateOrderStatus)
		protected.POST("/orders/:id/payments", middleware.RequirePermission(models.PaymentsWrite), orderHandler.AddOrderPayment)
		protected.DELETE("/orders/:id/payments/:paymentId", middleware.RequirePermission(models.PaymentsWrite), orderHandler.DeleteOrderPayment)
		protected.PATCH("/orders/:id/discounts", middleware.RequirePermission(models.DiscountsWrite), orderHandler.UpdateOrderDiscount)
		protected.DELETE("/orders/:id", middleware.RequirePermission(models.OrdersDelete), orderHandler.DeleteOrder)

		protected.GET("/catalogs", middleware.RequirePermission(models.CatalogsRead), catalogHandler.GetCatalogs)
		protected.POST("/catalogs", middleware.RequirePermission(models.CatalogsWrite), catalogHandler.CreateCatalog)
		protected.PUT("/catalogs/:id", middleware.RequirePermission(models.CatalogsWrite), catalogHandler.UpdateCatalog)
		protected.DELETE("/catalogs/:id", middleware.RequirePermission(models.CatalogsWrite), catalogHandler.DeleteCatalog)
		protected.GET("/catalogs/key/:key/items", middleware.RequirePermission(models.CatalogsRead), catalogHandler.GetCatalogItemsByKey)
		protected.GET("/catalogs/:id/items", middleware.RequirePermission(models.CatalogsRead), catalogHandler.GetCatalogItems)
		protected.POST("/catalogs/:id/items", middleware.RequirePermission(models.CatalogsWrite), catalogHandler.CreateCatalogItem)
		protected.PUT("/catalogs/:id/items/:itemId", middleware.RequirePermission(models.CatalogsWrite), catalogHandler.UpdateCatalogItem)
		protected.DELETE("/catalogs/:id/items/:itemId", middleware.RequirePermission(models.CatalogsWrite), catalogHandler.DeleteCatalogItem)
	}
}
