package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"almak-back/internal/aiorders"
	"almak-back/internal/database"
	"almak-back/internal/draftorders"
	"almak-back/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

var errDraftConflict = errors.New("draft conflict")
var errDraftBusy = errors.New("draft AI in progress")

type DraftHandler struct{}
type draftWrite struct {
	ID           string               `json:"id"`
	Kind         string               `json:"kind"`
	BaseRevision int                  `json:"baseRevision"`
	RequestID    string               `json:"requestId"`
	Document     draftorders.Document `json:"document"`
}
type draftImport struct {
	draftWrite
	SourceKey string                `json:"sourceKey"`
	OrderID   *uint                 `json:"orderId"`
	CreatedAt time.Time             `json:"createdAt"`
	UpdatedAt time.Time             `json:"updatedAt"`
	Messages  []models.DraftMessage `json:"messages"`
}

func decodeDraft(c *gin.Context, dst any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(dst) != nil {
		c.JSON(400, gin.H{"message": "Некорректный или слишком большой черновик"})
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		c.JSON(400, gin.H{"message": "Ожидался один документ"})
		return false
	}
	return true
}
func draftHash(doc draftorders.Document) string {
	data, _ := json.Marshal(doc)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func draftActor(c *gin.Context) uint { return c.MustGet("user").(models.User).ID }
func readDraft(db *gorm.DB, id string) (models.OrderDraft, error) {
	var row models.OrderDraft
	err := db.Preload("Creator", func(q *gorm.DB) *gorm.DB { return q.Select("id", "login", "name", "role") }).Preload("Editor", func(q *gorm.DB) *gorm.DB { return q.Select("id", "login", "name", "role") }).Preload("Messages", func(q *gorm.DB) *gorm.DB { return q.Order("created_at ASC, id ASC") }).First(&row, "id = ?", id).Error
	if row.Messages == nil {
		row.Messages = []models.DraftMessage{}
	}
	return row, err
}
func respondDraft(c *gin.Context, id string) {
	row, err := readDraft(draftDB().WithContext(c.Request.Context()), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"message": "Черновик не найден"})
		return
	}
	if err != nil {
		c.JSON(500, gin.H{"message": "Не удалось загрузить черновик"})
		return
	}
	c.JSON(200, row)
}
func draftError(c *gin.Context, err error, id string) {
	if errors.Is(err, errDraftBusy) {
		c.JSON(409, gin.H{"code": "AI_BUSY", "message": "Помощник ещё обрабатывает сообщение. Дождитесь ответа и повторите сохранение заказа."})
		return
	}
	if errors.Is(err, errDraftConflict) {
		row, _ := readDraft(draftDB(), id)
		c.JSON(409, gin.H{"code": "DRAFT_CONFLICT", "message": "Черновик изменён другим пользователем. Ваши правки сохранены в открытом окне.", "current": row})
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"message": "Черновик не найден"})
		return
	}
	c.JSON(500, gin.H{"message": "Не удалось сохранить черновик. Повторите попытку."})
}

func (h *DraftHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit < 1 || limit > 100 {
		limit = 100
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}
	q := draftDB().WithContext(c.Request.Context()).Model(&models.OrderDraft{})
	if c.Query("archived") == "true" {
		q = q.Where("state = ?", "archived")
	} else {
		q = q.Where("state <> ?", "archived")
	}
	if c.Query("mine") == "true" {
		actor := draftActor(c)
		q = q.Where("(created_by = ? OR imported_by = ?)", actor, actor)
	}
	var total int64
	if q.Count(&total).Error != nil {
		c.JSON(500, gin.H{"message": "Не удалось загрузить список"})
		return
	}
	var rows []models.OrderDraft
	if q.Preload("Creator", func(q *gorm.DB) *gorm.DB { return q.Select("id", "login", "name", "role") }).Preload("Editor", func(q *gorm.DB) *gorm.DB { return q.Select("id", "login", "name", "role") }).Preload("Messages", func(q *gorm.DB) *gorm.DB { return q.Order("created_at ASC, id ASC") }).Order("updated_at DESC, id ASC").Limit(limit).Offset(offset).Find(&rows).Error != nil {
		c.JSON(500, gin.H{"message": "Не удалось загрузить список"})
		return
	}
	if rows == nil {
		rows = []models.OrderDraft{}
	}
	c.JSON(200, gin.H{"items": rows, "total": total})
}
func (h *DraftHandler) Get(c *gin.Context) {
	if !draftorders.Identifier.MatchString(c.Param("id")) {
		c.JSON(400, gin.H{"message": "Некорректный ID"})
		return
	}
	respondDraft(c, c.Param("id"))
}

func (h *DraftHandler) Create(c *gin.Context) {
	var req draftWrite
	if !decodeDraft(c, &req) {
		return
	}
	if !draftorders.Identifier.MatchString(req.ID) || !draftorders.Identifier.MatchString(req.RequestID) || draftorders.Validate(req.Kind, req.Document) != nil {
		c.JSON(400, gin.H{"message": "Проверьте данные черновика"})
		return
	}
	actor := draftActor(c)
	row := models.OrderDraft{ID: req.ID, Kind: req.Kind, Revision: 1, State: "active", Document: req.Document, CreatedBy: &actor, UpdatedBy: actor, CreateRequest: req.RequestID}
	result := draftDB().Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		draftError(c, result.Error, req.ID)
		return
	}
	if result.RowsAffected == 0 {
		var existing models.OrderDraft
		if draftDB().First(&existing, "id = ?", req.ID).Error != nil {
			draftError(c, errors.New("create failed"), req.ID)
			return
		}
		if existing.CreateRequest != req.RequestID || existing.Revision != 1 || draftHash(existing.Document) != draftHash(req.Document) {
			draftError(c, errDraftConflict, req.ID)
			return
		}
	}
	respondDraft(c, req.ID)
}

func (h *DraftHandler) Update(c *gin.Context)  { h.mutate(c, "") }
func (h *DraftHandler) Archive(c *gin.Context) { h.mutate(c, "archive") }
func (h *DraftHandler) Restore(c *gin.Context) { h.mutate(c, "restore") }
func (h *DraftHandler) mutate(c *gin.Context, action string) {
	var req draftWrite
	if !decodeDraft(c, &req) {
		return
	}
	id := c.Param("id")
	if !draftorders.Identifier.MatchString(id) || !draftorders.Identifier.MatchString(req.RequestID) || req.BaseRevision < 1 || (action == "" && draftorders.Validate(req.Kind, req.Document) != nil) {
		c.JSON(400, gin.H{"message": "Проверьте данные черновика"})
		return
	}
	hash := action + ":" + draftHash(req.Document)
	err := draftDB().WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var row models.OrderDraft
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error; err != nil {
			return err
		}
		var done models.DraftMutation
		if err := tx.First(&done, "id = ?", req.RequestID).Error; err == nil {
			if done.DraftID != id || done.Hash != hash || done.Revision != row.Revision {
				return errDraftConflict
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.Revision != req.BaseRevision {
			return errDraftConflict
		}
		if action == "" {
			if row.State != "active" || row.Kind != req.Kind {
				return errDraftConflict
			}
			row.Document = req.Document
		} else if action == "archive" {
			row.State = "archived"
		} else {
			if row.State != "archived" {
				return errDraftConflict
			}
			row.State = "active"
			if row.OrderID != nil {
				row.State = "converted"
			}
		}
		row.Revision++
		row.UpdatedBy = draftActor(c)
		if err := tx.Model(&row).Select("document", "revision", "state", "updated_by", "updated_at").Updates(&row).Error; err != nil {
			return err
		}
		return tx.Create(&models.DraftMutation{ID: req.RequestID, DraftID: id, Hash: hash, Revision: row.Revision}).Error
	})
	if err != nil {
		draftError(c, err, id)
		return
	}
	respondDraft(c, id)
}

func (h *DraftHandler) Import(c *gin.Context) {
	var req draftImport
	if !decodeDraft(c, &req) {
		return
	}
	if !draftorders.Identifier.MatchString(req.ID) || len(req.SourceKey) < 1 || len(req.SourceKey) > 200 || draftorders.Validate(req.Kind, req.Document) != nil || len(req.Messages) > 500 {
		c.JSON(400, gin.H{"message": "Не удалось проверить локальную запись"})
		return
	}
	for _, m := range req.Messages {
		if (m.Role != "user" && m.Role != "assistant") || len([]rune(m.Text)) > 10000 {
			c.JSON(400, gin.H{"message": "Некорректная история сообщений"})
			return
		}
	}
	actor := draftActor(c)
	key := req.SourceKey
	data, _ := json.Marshal(struct {
		Kind     string
		Document draftorders.Document
		Messages []models.DraftMessage
		OrderID  *uint
	}{req.Kind, req.Document, req.Messages, req.OrderID})
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	err := draftDB().Transaction(func(tx *gorm.DB) error {
		row := models.OrderDraft{ID: req.ID, Kind: req.Kind, Revision: 1, State: "active", Document: req.Document, UpdatedBy: actor, ImportedBy: &actor, ImportKey: &key, ImportHash: hash, OrderID: req.OrderID}
		if req.OrderID != nil {
			if *req.OrderID == 0 {
				return errDraftConflict
			}
			row.State = "converted"
		}
		if !req.CreatedAt.IsZero() && req.CreatedAt.Before(time.Now().Add(24*time.Hour)) {
			row.CreatedAt = req.CreatedAt
		}
		if !req.UpdatedAt.IsZero() && req.UpdatedAt.Before(time.Now().Add(24*time.Hour)) {
			row.UpdatedAt = req.UpdatedAt
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var existing models.OrderDraft
			if err := tx.First(&existing, "id = ? OR import_key = ?", req.ID, key).Error; err != nil {
				return err
			}
			if existing.ImportKey == nil || existing.ImportHash != hash {
				return errDraftConflict
			}
			req.ID = existing.ID
			return nil
		}
		for _, m := range req.Messages {
			m.ID = 0
			m.DraftID = row.ID
			m.AuthorID = nil
			m.RequestID = ""
			if err := tx.Create(&m).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		draftError(c, err, req.ID)
		return
	}
	respondDraft(c, req.ID)
}

func (h *DraftHandler) Undo(c *gin.Context) {
	var req draftWrite
	if !decodeDraft(c, &req) {
		return
	}
	id := c.Param("id")
	if !draftorders.Identifier.MatchString(id) || !draftorders.Identifier.MatchString(req.RequestID) {
		c.JSON(400, gin.H{"message": "Некорректный запрос"})
		return
	}
	err := draftDB().Transaction(func(tx *gorm.DB) error {
		var row models.OrderDraft
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error; err != nil {
			return err
		}
		var done models.DraftMutation
		if tx.First(&done, "id = ?", req.RequestID).Error == nil {
			if done.DraftID == id && done.Hash == "undo" && done.Revision == row.Revision {
				return nil
			}
			return errDraftConflict
		}
		if row.Revision != req.BaseRevision || row.State != "active" || row.Document.LastAppliedChange == nil {
			return errDraftConflict
		}
		row.Document.Order = &row.Document.LastAppliedChange.Order
		row.Document.FieldSources = row.Document.LastAppliedChange.FieldSources
		row.Document.LastAppliedChange = nil
		row.Document.Changes = []aiorders.Change{}
		row.Document.Questions = []string{}
		row.Revision++
		row.UpdatedBy = draftActor(c)
		if err := tx.Model(&row).Select("document", "revision", "updated_by", "updated_at").Updates(&row).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.DraftMessage{DraftID: id, Role: "assistant", Text: "Последнее изменение отменено."}).Error; err != nil {
			return err
		}
		return tx.Create(&models.DraftMutation{ID: req.RequestID, DraftID: id, Hash: "undo", Revision: row.Revision}).Error
	})
	if err != nil {
		draftError(c, err, id)
		return
	}
	respondDraft(c, id)
}

func draftDB() *gorm.DB {
	return database.DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}
