package templates

import (
	"fmt"
	"log/slog"
	"net/http"
)

func RenderResponseMessage(w http.ResponseWriter, statusCode int, message string) {
	slog.Info("response message", "status", statusCode, "message", message)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)

	var messageClass string
	if statusCode >= http.StatusBadRequest {
		messageClass = "error"
	} else if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		messageClass = "success"
	}

	html := fmt.Sprintf(`<div class="alert alert-%s"><span class="alert-text">%s</span></div>`, messageClass, message)
	slog.Info("response message", "html", html)
	if _, err := w.Write([]byte(html)); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}
