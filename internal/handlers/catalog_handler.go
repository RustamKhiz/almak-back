package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"almak-back/internal/database"
	"almak-back/internal/models"

	"github.com/gin-gonic/gin"
)

type CatalogHandler struct{}

func NewCatalogHandler() *CatalogHandler {
	return &CatalogHandler{}
}

type catalogRequest struct {
	Name string  `json:"name" binding:"required"`
	Key  *string `json:"key"`
}

type catalogItemRequest struct {
	Value string `json:"value" binding:"required"`
}

type catalogResponse struct {
	ID         uint    `json:"id"`
	Key        *string `json:"key"`
	Name       string  `json:"name"`
	ItemsCount int64   `json:"itemsCount"`
}

func (h *CatalogHandler) GetCatalogs(c *gin.Context) {
	var catalogs []models.Catalog
	if err := database.DB.Find(&catalogs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch catalogs"})
		return
	}

	result := make([]catalogResponse, 0, len(catalogs))
	for _, cat := range catalogs {
		var count int64
		database.DB.Model(&models.CatalogItem{}).Where("catalog_id = ?", cat.ID).Count(&count)
		result = append(result, catalogResponse{ID: cat.ID, Key: cat.Key, Name: cat.Name, ItemsCount: count})
	}
	c.JSON(http.StatusOK, result)
}

func (h *CatalogHandler) CreateCatalog(c *gin.Context) {
	var req catalogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name cannot be empty"})
		return
	}

	var key *string
	if req.Key != nil {
		trimmed := strings.TrimSpace(*req.Key)
		if trimmed != "" {
			key = &trimmed
		}
	}

	catalog := models.Catalog{Name: name, Key: key}
	if err := database.DB.Create(&catalog).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Catalog with this name already exists"})
		return
	}
	c.JSON(http.StatusCreated, catalogResponse{ID: catalog.ID, Key: catalog.Key, Name: catalog.Name, ItemsCount: 0})
}

func (h *CatalogHandler) UpdateCatalog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	var req catalogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name cannot be empty"})
		return
	}

	var catalog models.Catalog
	if err := database.DB.First(&catalog, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Catalog not found"})
		return
	}
	catalog.Name = name
	if req.Key != nil {
		trimmed := strings.TrimSpace(*req.Key)
		if trimmed != "" {
			catalog.Key = &trimmed
		} else {
			catalog.Key = nil
		}
	}
	if err := database.DB.Save(&catalog).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Catalog with this name already exists"})
		return
	}

	var count int64
	database.DB.Model(&models.CatalogItem{}).Where("catalog_id = ?", catalog.ID).Count(&count)
	c.JSON(http.StatusOK, catalogResponse{ID: catalog.ID, Key: catalog.Key, Name: catalog.Name, ItemsCount: count})
}

func (h *CatalogHandler) DeleteCatalog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	if err := database.DB.Delete(&models.Catalog{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete catalog"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *CatalogHandler) GetCatalogItemsByKey(c *gin.Context) {
	key := strings.TrimSpace(c.Param("key"))
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid key"})
		return
	}
	var catalog models.Catalog
	if err := database.DB.Where("key = ?", key).First(&catalog).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Catalog not found"})
		return
	}
	var items []models.CatalogItem
	if err := database.DB.Where("catalog_id = ?", catalog.ID).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch items"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *CatalogHandler) GetCatalogItems(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	if err := database.DB.First(&models.Catalog{}, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Catalog not found"})
		return
	}
	var items []models.CatalogItem
	if err := database.DB.Where("catalog_id = ?", id).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch items"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *CatalogHandler) CreateCatalogItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	if err := database.DB.First(&models.Catalog{}, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Catalog not found"})
		return
	}
	var req catalogItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Value is required"})
		return
	}
	value := strings.TrimSpace(req.Value)
	if value == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Value cannot be empty"})
		return
	}

	item := models.CatalogItem{CatalogID: uint(id), Value: value}
	if err := database.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create item"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *CatalogHandler) UpdateCatalogItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	itemID, err := strconv.Atoi(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item id"})
		return
	}
	var req catalogItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Value is required"})
		return
	}
	value := strings.TrimSpace(req.Value)
	if value == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Value cannot be empty"})
		return
	}

	var item models.CatalogItem
	if err := database.DB.Where("id = ? AND catalog_id = ?", itemID, id).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}
	item.Value = value
	if err := database.DB.Save(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item"})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *CatalogHandler) DeleteCatalogItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id"})
		return
	}
	itemID, err := strconv.Atoi(c.Param("itemId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item id"})
		return
	}
	result := database.DB.Where("id = ? AND catalog_id = ?", itemID, id).Delete(&models.CatalogItem{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete item"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}
	c.Status(http.StatusNoContent)
}
