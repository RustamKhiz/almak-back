package handlers

import (
	"almak-back/internal/aiorders"
	"almak-back/internal/draftorders"
	"almak-back/internal/models"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

func (h *AiOrderHandler) SendDraftMessage(c *gin.Context) {
	var input struct {
		RequestID    string `json:"requestId"`
		BaseRevision int    `json:"baseRevision"`
		Text         string `json:"text"`
	}
	if !decodeDraft(c, &input) {
		return
	}
	id := c.Param("id")
	actor := draftActor(c)
	input.Text = strings.TrimSpace(input.Text)
	if !draftorders.Identifier.MatchString(id) || !draftorders.Identifier.MatchString(input.RequestID) || strings.TrimSpace(input.Text) == "" || len([]rune(input.Text)) > 10000 {
		c.JSON(400, gin.H{"message": "Проверьте текст сообщения"})
		return
	}
	already := false
	var snapshot models.OrderDraft
	busy := errors.New("AI busy")
	limit := errors.New("message limit")
	err := draftDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&snapshot, "id = ?", id).Error; err != nil {
			return err
		}
		if !canAccessDraft(c, snapshot) {
			return gorm.ErrRecordNotFound
		}
		if snapshot.Kind != "ai" || snapshot.State != "active" || snapshot.Document.Order == nil || (snapshot.Document.EditorState != nil && snapshot.Document.EditorState.Accepted) {
			return errDraftConflict
		}
		var job models.DraftAIRequest
		aliasID := ""
		jobErr := tx.First(&job, "id = ?", input.RequestID).Error
		if jobErr == nil && job.CanonicalID != "" {
			if job.DraftID != id || job.UserID != actor || job.Text != input.Text {
				return errDraftConflict
			}
			canonical := job.CanonicalID
			job = models.DraftAIRequest{}
			jobErr = tx.First(&job, "id = ?", canonical).Error
			input.RequestID = canonical
		}
		if errors.Is(jobErr, gorm.ErrRecordNotFound) {
			var previous models.DraftAIRequest
			previousErr := tx.Where("draft_id = ? AND user_id = ? AND text = ? AND canonical_id = ?", id, actor, input.Text, "").Order("created_at DESC, id DESC").First(&previous).Error
			if previousErr == nil && previous.Status != "done" {
				aliasID = input.RequestID
				job = previous
				input.RequestID = job.ID
				jobErr = nil
			} else if previousErr != nil && !errors.Is(previousErr, gorm.ErrRecordNotFound) {
				return previousErr
			}
		}
		if jobErr == nil {
			if job.DraftID != id || job.UserID != actor || job.Text != input.Text {
				return errDraftConflict
			}
			if job.Status == "done" {
				already = true
				return nil
			}
			if job.Status == "working" && time.Since(job.UpdatedAt) < time.Minute {
				return busy
			}
		} else if !errors.Is(jobErr, gorm.ErrRecordNotFound) {
			return jobErr
		}
		if snapshot.Revision != input.BaseRevision {
			return errDraftConflict
		}
		var working int64
		if err := tx.Model(&models.DraftAIRequest{}).Where("draft_id = ? AND status = ? AND updated_at > ?", id, "working", time.Now().Add(-time.Minute)).Count(&working).Error; err != nil {
			return err
		}
		if working > 0 {
			return busy
		}
		if aliasID != "" {
			if err := tx.Create(&models.DraftAIRequest{ID: aliasID, CanonicalID: job.ID, DraftID: id, UserID: actor, Text: input.Text, BaseRevision: snapshot.Revision, Status: "alias"}).Error; err != nil {
				return err
			}
		}
		if job.ID == "" {
			var count int64
			if err := tx.Model(&models.DraftMessage{}).Where("draft_id = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count >= 500 {
				return limit
			}
			var turns int64
			if err := tx.Model(&models.DraftMessage{}).Where("draft_id = ? AND role = ?", id, "user").Count(&turns).Error; err != nil {
				return err
			}
			if turns >= 20 {
				return limit
			}
			job = models.DraftAIRequest{ID: input.RequestID, DraftID: id, UserID: actor, Text: input.Text, BaseRevision: snapshot.Revision, Status: "working"}
			if err := tx.Create(&job).Error; err != nil {
				return err
			}
			return tx.Create(&models.DraftMessage{DraftID: id, Role: "user", Text: input.Text, AuthorID: &actor, RequestID: input.RequestID}).Error
		}
		return tx.Model(&job).Updates(map[string]any{"status": "working", "base_revision": snapshot.Revision}).Error
	})
	if err != nil {
		if errors.Is(err, limit) {
			c.JSON(413, gin.H{"code": "MESSAGE_LIMIT", "message": "Достигнут лимит уточнений. Продолжите заполнение в форме."})
		} else if errors.Is(err, busy) {
			c.JSON(409, gin.H{"code": "AI_BUSY", "message": "Сообщение уже обрабатывается. Обновите переписку через несколько секунд."})
		} else {
			draftError(c, err, id)
		}
		return
	}
	if already {
		respondDraft(c, id)
		return
	}
	var messages []models.DraftMessage
	if err = draftDB().Where("draft_id = ?", id).Order("created_at ASC, id ASC").Find(&messages).Error; err != nil {
		c.JSON(500, gin.H{"message": "Не удалось загрузить переписку"})
		return
	}
	history := []aiorders.Message{}
	mode := "initial"
	for _, m := range messages {
		if m.Role == "assistant" {
			mode = "refine"
		}
		if m.RequestID == input.RequestID {
			continue
		}
		history = append(history, aiorders.Message{Role: m.Role, Text: m.Text})
	}
	if len(history) > 40 {
		history = history[len(history)-40:]
	}
	if snapshot.TargetOrderID != nil {
		mode = "edit_order"
	}
	req := aiorders.Request{RequestID: input.RequestID, DraftID: id, BaseRevision: snapshot.Revision, SchemaVersion: 1, Mode: mode, Text: input.Text, CurrentDraft: *snapshot.Document.Order, FieldSources: snapshot.Document.FieldSources, History: history}
	response, failure := h.generate(c.Request.Context(), fmt.Sprint(actor), req)
	if failure != nil {
		draftDB().Model(&models.DraftAIRequest{}).Where("id = ?", input.RequestID).Update("status", "error")
		c.JSON(failure.Status, gin.H{"code": failure.Code, "message": failure.Message})
		return
	}
	err = draftDB().Transaction(func(tx *gorm.DB) error {
		var row models.OrderDraft
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", id).Error; err != nil {
			return err
		}
		if row.Revision != snapshot.Revision || row.State != "active" {
			return errDraftConflict
		}
		previous := *row.Document.Order
		row.Document.LastAppliedChange = &draftorders.Previous{Order: previous, FieldSources: row.Document.FieldSources}
		sources := map[string]aiorders.Source{}
		for p, v := range row.Document.FieldSources {
			sources[p] = v
		}
		for _, change := range response.Changes {
			sources[change.Path] = aiorders.Source{Text: string([]rune(input.Text)[:min(300, len([]rune(input.Text)))]), Manual: false}
		}
		row.Document.FieldSources = sources
		row.Document.Order = &response.ProposedDraft
		row.Document.Questions = []string{}
		row.Document.Changes = response.Changes
		if row.EditBase != nil {
			if normalizeEditProposal(*row.EditBase, previous, row.Document.Order) {
				row.Document.Order.Notes = append(row.Document.Order.Notes, "Размеры и количество доборов и обшивки изменяйте в обычном редакторе.")
			}
			row.Document.Changes = editChanges(*row.EditBase, *row.Document.Order)
		}
		if strings.TrimSpace(row.Document.PendingText) == input.Text {
			row.Document.PendingText = ""
		}
		row.Revision++
		row.UpdatedBy = actor
		if err := tx.Model(&row).Select("document", "revision", "updated_by", "updated_at").Updates(&row).Error; err != nil {
			return err
		}
		text := draftReply(mode, previous, *row.Document.Order, len(row.Document.Changes))
		if err := tx.Create(&models.DraftMessage{DraftID: id, Role: "assistant", Text: text, RequestID: input.RequestID}).Error; err != nil {
			return err
		}
		return tx.Model(&models.DraftAIRequest{}).Where("id = ?", input.RequestID).Update("status", "done").Error
	})
	if err != nil {
		draftDB().Model(&models.DraftAIRequest{}).Where("id = ?", input.RequestID).Update("status", "conflict")
		draftError(c, err, id)
		return
	}
	respondDraft(c, id)
}
