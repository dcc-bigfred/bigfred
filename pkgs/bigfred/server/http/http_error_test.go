package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
)

func testErrorLogger() (*logrus.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	log := logrus.New()
	log.SetOutput(&buf)
	log.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		DisableColors:    true,
	})
	return log, &buf
}

func testErrorRouter(log *logrus.Logger, pattern string, h http.HandlerFunc) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(withRequestLog(log))
	r.Use(recoverer)
	r.Get(pattern, h)
	return r
}

func TestWriteJSONErrorCauseLogsStackNotBody(t *testing.T) {
	log, buf := testErrorLogger()
	cause := errors.New("redis down")
	handler := testErrorRouter(log, "/boom", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONErrorCause(w, http.StatusInternalServerError, "internal_error", cause)
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
	body := rec.Body.String()
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("body=%s err=%v", body, err)
	}
	if payload["error"] != "internal_error" {
		t.Fatalf("body=%s", body)
	}
	if strings.Contains(body, "redis down") {
		t.Fatalf("error leaked into body: %s", body)
	}
	if strings.Contains(body, "goroutine") || strings.Contains(body, "writeJSONErrorCause") {
		t.Fatalf("stack leaked into body: %s", body)
	}

	logged := buf.String()
	if !strings.Contains(logged, "redis down") {
		t.Fatalf("log missing cause: %s", logged)
	}
	if !strings.Contains(logged, "internal_error") {
		t.Fatalf("log missing code: %s", logged)
	}
	if !strings.Contains(logged, "writeJSONErrorCause") {
		t.Fatalf("log missing stack: %s", logged)
	}
	if !strings.Contains(logged, "goroutine") {
		t.Fatalf("log missing goroutine: %s", logged)
	}
	if !strings.Contains(logged, "path=/boom") {
		t.Fatalf("log missing path: %s", logged)
	}
}

func TestWriteJSONErrorDoesNotLog4xx(t *testing.T) {
	log, buf := testErrorLogger()
	handler := testErrorRouter(log, "/bad", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/bad", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"error":"invalid_body"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if buf.Len() != 0 {
		t.Fatalf("4xx must not log: %s", buf.String())
	}
}

func TestRecovererLogsPanicNotBody(t *testing.T) {
	log, buf := testErrorLogger()
	handler := testErrorRouter(log, "/panic", func(http.ResponseWriter, *http.Request) {
		panic("kaboom")
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"error":"internal_error"`) {
		t.Fatalf("body=%s", body)
	}
	if strings.Contains(body, "kaboom") {
		t.Fatalf("panic leaked into body: %s", body)
	}
	if strings.Contains(body, "goroutine") {
		t.Fatalf("stack leaked into body: %s", body)
	}

	logged := buf.String()
	if !strings.Contains(logged, "kaboom") {
		t.Fatalf("log missing panic: %s", logged)
	}
	if !strings.Contains(logged, "goroutine") {
		t.Fatalf("log missing stack: %s", logged)
	}
	if !strings.Contains(logged, "http panic") {
		t.Fatalf("log missing panic marker: %s", logged)
	}
}

func TestRecovererRepanicsAbortHandler(t *testing.T) {
	log, buf := testErrorLogger()
	handler := testErrorRouter(log, "/abort", func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	})

	rec := httptest.NewRecorder()
	defer func() {
		recov := recover()
		if recov != http.ErrAbortHandler {
			t.Fatalf("recover=%v want ErrAbortHandler", recov)
		}
		if rec.Code != 200 {
			t.Fatalf("status=%d, abort must not write 500", rec.Code)
		}
		if buf.Len() != 0 {
			t.Fatalf("abort must not log: %s", buf.String())
		}
	}()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/abort", nil))
	t.Fatal("expected panic")
}
