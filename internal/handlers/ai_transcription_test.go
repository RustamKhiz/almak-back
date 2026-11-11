package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"almak-back/internal/config"
	"github.com/gin-gonic/gin"
)

func TestTranscriptionRejectsOversizedAudio(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"ffmpeg", "whisper-cli", "model.bin"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	h := NewAiOrderHandler(config.Config{AIOrderDraftsEnabled: true, WhisperDirectory: dir})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Set("user_id", 1)
	c.Request = httptest.NewRequest(http.MethodPost, "/ai/order-drafts/transcribe", bytes.NewReader(make([]byte, 900*1024+1)))
	c.Request.Header.Set("Content-Type", "audio/webm;codecs=opus")
	h.Transcribe(c)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("unexpected response %d %s", response.Code, response.Body.String())
	}
}

func TestTranscriptionWithWhisper(t *testing.T) {
	dir := os.Getenv("ALMAK_TEST_WHISPER_DIRECTORY")
	sample := os.Getenv("ALMAK_TEST_DICTATION_AUDIO")
	if dir == "" || sample == "" {
		t.Skip("set a Whisper directory and a WebM recording to test local transcription")
	}
	audio, err := os.ReadFile(sample)
	if err != nil {
		t.Fatal(err)
	}
	h := NewAiOrderHandler(config.Config{AIOrderDraftsEnabled: true, WhisperDirectory: dir})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Set("user_id", 1)
	c.Request = httptest.NewRequest(http.MethodPost, "/ai/order-drafts/transcribe", bytes.NewReader(audio))
	c.Request.Header.Set("Content-Type", "audio/webm;codecs=opus")
	h.Transcribe(c)
	var transcript struct{ Text string }
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &transcript) != nil || transcript.Text == "" {
		t.Fatalf("unexpected response %d %s", response.Code, response.Body.String())
	}
	t.Log(transcript.Text)
}
