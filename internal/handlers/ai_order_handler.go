package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"almak-back/internal/aiorders"
	"almak-back/internal/config"
	"almak-back/internal/database"
	"github.com/gin-gonic/gin"
)

type aiRate struct {
	Start time.Time
	Count int
}
type AiOrderHandler struct {
	cfg    config.Config
	client *http.Client
	slots  chan struct{}
	mu     sync.Mutex
	rates  map[string]aiRate
}

func NewAiOrderHandler(cfg config.Config) *AiOrderHandler {
	return &AiOrderHandler{cfg: cfg, client: &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, slots: make(chan struct{}, 3), rates: map[string]aiRate{}}
}
func (h *AiOrderHandler) allowed(user string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	for k, r := range h.rates {
		if now.Sub(r.Start) > time.Minute {
			delete(h.rates, k)
		}
	}
	r := h.rates[user]
	if r.Start.IsZero() {
		r.Start = now
	}
	if r.Count >= 10 {
		return false
	}
	r.Count++
	h.rates[user] = r
	return true
}
func (h *AiOrderHandler) Parse(c *gin.Context) {
	requestID := ""
	fail := func(status int, code, message string) {
		c.JSON(status, gin.H{"code": code, "message": message, "requestId": requestID})
	}
	if !h.cfg.AIOrderDraftsEnabled || h.cfg.DeepSeekAPIKey == "" {
		fail(503, "AI_DISABLED", "Помощник пока не подключён. Текст можно сохранить и заполнить заказ вручную.")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10)
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			fail(413, "REQUEST_TOO_LARGE", "Слишком большой черновик.")
		} else {
			fail(400, "INVALID_REQUEST", "Не удалось прочитать запрос.")
		}
		return
	}
	var req aiorders.Request
	if err = aiorders.DecodeStrict(data, &req); err != nil {
		fail(400, "INVALID_REQUEST", "Некорректный формат черновика.")
		return
	}
	requestID = req.RequestID
	if err = aiorders.ValidateRequest(req); err != nil {
		fail(400, "INVALID_REQUEST", "Проверьте объём текста и данные черновика.")
		return
	}
	if !h.allowed(fmt.Sprint(c.MustGet("user_id"))) {
		c.Header("Retry-After", "60")
		fail(429, "RATE_LIMIT", "Слишком много запросов. Повторите через минуту.")
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		fail(429, "BUSY", "Помощник занят. Повторите немного позже.")
		return
	}
	catalogs := map[string][]string{}
	if database.DB != nil {
		var rows []struct {
			Key   string
			Value string
		}
		if err := database.DB.WithContext(c.Request.Context()).Table("catalog_items").Select("catalogs.key, catalog_items.value").Joins("JOIN catalogs ON catalogs.id = catalog_items.catalog_id").Where("catalogs.key IN ?", []string{"suppliers", "entrance_door_painting", "entrance_door_panel_color", "interior_door_coverings", "molding_coverings", "extension_coverings", "capital_coverings", "paneling_coverings"}).Limit(1001).Scan(&rows).Error; err != nil {
			fail(502, "CATALOGS_UNAVAILABLE", "Не удалось загрузить справочники. Повторите запрос.")
			return
		}
		if len(rows) > 1000 {
			fail(413, "CATALOGS_TOO_LARGE", "Справочники слишком велики для разбора. Обратитесь к администратору.")
			return
		}
		// Only known, relevant string dictionaries; no other application data is sent.
		for _, row := range rows {
			switch row.Key {
			case "suppliers", "entrance_door_painting", "entrance_door_panel_color", "interior_door_coverings", "molding_coverings", "extension_coverings", "capital_coverings", "paneling_coverings":
				catalogs[row.Key] = append(catalogs[row.Key], row.Value)
			}
		}
	}
	loc := time.FixedZone("Europe/Moscow", 3*60*60)
	input, _ := json.Marshal(gin.H{"request": req, "catalogs": catalogs, "contextDate": time.Now().In(loc).Format("2006-01-02"), "timezone": "Europe/Moscow"})
	if len(input) > 160<<10 {
		fail(413, "CONTEXT_TOO_LARGE", "Контекст слишком большой. Продолжите заполнение в форме.")
		return
	}
	payload, _ := json.Marshal(gin.H{"model": h.cfg.DeepSeekModel, "messages": []gin.H{{"role": "system", "content": aiorders.Prompt + "\nSCHEMA JSON:\n" + string(aiorders.SchemaJSON)}, {"role": "user", "content": string(input)}}, "response_format": gin.H{"type": "json_object"}, "thinking": gin.H{"type": "disabled"}, "max_tokens": 8192})
	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()
	upstream, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(h.cfg.DeepSeekBaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		fail(503, "AI_CONFIGURATION", "Проверьте настройки помощника.")
		return
	}
	upstream.Header.Set("Content-Type", "application/json")
	upstream.Header.Set("Authorization", "Bearer "+h.cfg.DeepSeekAPIKey)
	started := time.Now()
	res, err := h.client.Do(upstream)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			fail(504, "AI_TIMEOUT", "Помощник не успел ответить. Черновик сохранён; можно повторить.")
		} else if c.Request.Context().Err() == nil {
			fail(502, "AI_UNAVAILABLE", "Не удалось получить ответ помощника. Можно повторить.")
		}
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		if res.StatusCode == 429 {
			fail(429, "PROVIDER_LIMIT", "Достигнут лимит провайдера. Повторите позже.")
		} else {
			fail(502, "AI_UNAVAILABLE", "Провайдер не выполнил запрос. Проверьте подключение помощника.")
		}
		return
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		fail(502, "AI_INVALID_RESPONSE", "Получен некорректный ответ. Предыдущие данные сохранены.")
		return
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &completion) != nil || len(completion.Choices) != 1 || completion.Choices[0].FinishReason != "stop" {
		fail(502, "AI_INVALID_RESPONSE", "Ответ неполный. Предыдущие данные сохранены; можно повторить.")
		return
	}
	var result aiorders.Result
	if aiorders.DecodeStrict([]byte(completion.Choices[0].Message.Content), &result) != nil || aiorders.ValidateResult(req, result) != nil {
		fail(502, "AI_INVALID_RESPONSE", "Ответ не прошёл проверку. Предыдущие данные сохранены; можно повторить или исправить вручную.")
		return
	}
	// No raw input, provider body, or credentials in logs.
	fmt.Printf("ai_order_parse request=%s model=%s duration_ms=%d tokens=%d result=ok\n", requestID, h.cfg.DeepSeekModel, time.Since(started).Milliseconds(), completion.Usage.TotalTokens)
	c.JSON(200, aiorders.Response{RequestID: req.RequestID, DraftID: req.DraftID, BaseRevision: req.BaseRevision, SchemaVersion: 1, ProposedDraft: result.Order, Questions: result.Questions, Issues: aiorders.Issues(result.Order), Changes: aiorders.Changes(req.CurrentDraft, result.Order)})
}
