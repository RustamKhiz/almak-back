package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"almak-back/internal/aiorders"
	"almak-back/internal/config"
	"github.com/gin-gonic/gin"
)

func TestAIParseProviderValidationAndFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, content, finish string
		status, want          int
	}{
		{"valid", `{"order":{"header":{"name":"Магомед"},"items":[],"notes":[]},"questions":["Какие двери?"]}`, "stop", 200, 200},
		{"truncated", `{"order":`, "length", 200, 502},
		{"unknown field", `{"order":{"header":{"apiKey":"no"},"items":[],"notes":[]},"questions":[]}`, "stop", 200, 502},
		{"provider limit", "", "", 429, 429},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("missing provider credential")
				}
				var request struct {
					Messages       []struct{ Role, Content string }
					ResponseFormat map[string]string `json:"response_format"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if len(request.Messages) != 2 || request.Messages[0].Role != "system" || !strings.Contains(request.Messages[0].Content, "САНТИМЕТРАМИ") {
					t.Error("missing fixed prompt")
				}
				w.WriteHeader(test.status)
				_ = json.NewEncoder(w).Encode(gin.H{"choices": []gin.H{{"message": gin.H{"content": test.content}, "finish_reason": test.finish}}})
			}))
			defer provider.Close()
			handler := NewAiOrderHandler(config.Config{AIOrderDraftsEnabled: true, DeepSeekAPIKey: "test-key", DeepSeekModel: "test-model", DeepSeekBaseURL: provider.URL})
			router := gin.New()
			router.POST("/parse", func(c *gin.Context) { c.Set("user_id", 1); handler.Parse(c) })
			input := aiorders.Request{RequestID: "req-1", DraftID: "draft-1", SchemaVersion: 1, Mode: "initial", Text: "Магомед", CurrentDraft: aiorders.Order{Header: map[string]any{}, Items: []aiorders.Item{}, Notes: []string{}}}
			body, _ := json.Marshal(input)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/parse", strings.NewReader(string(body)))
			router.ServeHTTP(rec, req)
			if rec.Code != test.want {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "test-key") {
				t.Fatal("credential leaked")
			}
			if test.want == 200 {
				var response aiorders.Response
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || len(response.Questions) != 0 {
					t.Fatal("provider questions reached the client")
				}
			}
		})
	}
}
func TestAIDisabledAndLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewAiOrderHandler(config.Config{})
	r := gin.New()
	r.POST("/parse", h.Parse)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/parse", strings.NewReader("{}")))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	for i := 0; i < 10; i++ {
		if !h.allowed("one") {
			t.Fatal("unexpected rate limit")
		}
	}
	if h.allowed("one") {
		t.Fatal("limit not enforced")
	}
	if !h.allowed("two") {
		t.Fatal("users share rate bucket")
	}
}
