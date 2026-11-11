package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *AiOrderHandler) transcriptionEnabled() bool {
	if !h.cfg.AIOrderDraftsEnabled || h.cfg.WhisperDirectory == "" {
		return false
	}
	for _, name := range []string{"ffmpeg", "whisper-cli", "model.bin"} {
		info, err := os.Stat(filepath.Join(h.cfg.WhisperDirectory, name))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func (h *AiOrderHandler) TranscriptionStatus(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"enabled": h.transcriptionEnabled()})
}

func (h *AiOrderHandler) Transcribe(c *gin.Context) {
	fail := func(status int, message string) { c.JSON(status, gin.H{"message": message}) }
	if !h.transcriptionEnabled() {
		fail(http.StatusServiceUnavailable, "Голосовой ввод пока не подключён.")
		return
	}
	mediaType, _, _ := mime.ParseMediaType(c.GetHeader("Content-Type"))
	inputFormat := map[string]string{"audio/webm": "matroska", "audio/mp4": "mov", "audio/ogg": "ogg"}[mediaType]
	if inputFormat == "" {
		fail(http.StatusUnsupportedMediaType, "Формат записи не поддерживается.")
		return
	}
	if !h.allowed(fmt.Sprint(c.MustGet("user_id"))) {
		fail(http.StatusTooManyRequests, "Слишком много запросов. Повторите через минуту.")
		return
	}
	select {
	case h.voiceSlot <- struct{}{}:
		defer func() { <-h.voiceSlot }()
	default:
		fail(http.StatusTooManyRequests, "Сейчас распознаётся другая запись. Попробуйте немного позже.")
		return
	}

	audio, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 900*1024))
	if err != nil || len(audio) == 0 {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(http.StatusRequestEntityTooLarge, "Запись слишком большая.")
		} else {
			fail(http.StatusBadRequest, "Запись пуста или не читается.")
		}
		return
	}

	dir, err := os.MkdirTemp("", "almak-dictation-")
	if err != nil {
		fail(http.StatusInternalServerError, "Не удалось подготовить запись.")
		return
	}
	defer os.RemoveAll(dir)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 75*time.Second)
	defer cancel()
	wav := filepath.Join(dir, "recording.wav")
	decode := exec.CommandContext(ctx, filepath.Join(h.cfg.WhisperDirectory, "ffmpeg"),
		"-nostdin", "-hide_banner", "-loglevel", "error", "-protocol_whitelist", "pipe",
		"-threads", "1", "-f", inputFormat, "-i", "pipe:0", "-vn", "-map", "0:a:0",
		"-filter_threads", "1", "-t", "60", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", wav)
	decode.Stdin = bytes.NewReader(audio)
	if err := decode.Run(); err != nil {
		fail(http.StatusUnprocessableEntity, "Не удалось прочитать звук из записи.")
		return
	}

	output := filepath.Join(dir, "transcript")
	recognize := exec.CommandContext(ctx, filepath.Join(h.cfg.WhisperDirectory, "whisper-cli"),
		"-m", filepath.Join(h.cfg.WhisperDirectory, "model.bin"), "-f", wav, "-l", "ru",
		"-t", "1", "-bs", "1", "-bo", "1", "-nt", "-otxt", "-of", output)
	if err := recognize.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 75 {
			fail(http.StatusServiceUnavailable, "Сервер сейчас занят. Попробуйте немного позже.")
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			fail(http.StatusGatewayTimeout, "Распознавание заняло слишком много времени. Попробуйте более короткую запись.")
		} else {
			fail(http.StatusServiceUnavailable, "Не удалось распознать запись. Попробуйте ещё раз.")
		}
		return
	}

	transcript, err := os.ReadFile(output + ".txt")
	if err != nil {
		fail(http.StatusServiceUnavailable, "Не удалось получить текст записи.")
		return
	}
	text := strings.TrimSpace(string(transcript))
	if text == "" {
		fail(http.StatusUnprocessableEntity, "Речь не распознана. Попробуйте записать ещё раз.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"text": text})
}
