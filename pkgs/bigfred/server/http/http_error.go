package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
)

type requestLogCtxKey struct{}

// responseLogWriter carries the request and logger so writeJSONError can
// log method/path/request_id without taking *http.Request at every call
// site. Hijack/Flush are forwarded so WebSocket upgrades keep working.
type responseLogWriter struct {
	http.ResponseWriter
	r   *http.Request
	log *logrus.Logger
}

func (w *responseLogWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseLogWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *responseLogWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("http.Hijacker not supported")
	}
	return h.Hijack()
}

func withRequestLog(log *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if log != nil {
				r = r.WithContext(context.WithValue(r.Context(), requestLogCtxKey{}, log))
			}
			next.ServeHTTP(&responseLogWriter{ResponseWriter: w, r: r, log: log}, r)
		})
	}
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			logHTTPPanic(w, r, rec)
			writeJSONErrorBody(w, http.StatusInternalServerError, "internal_error")
		}()
		next.ServeHTTP(w, r)
	})
}

// writeJSONError renders {"error": "..."} with the given status.
// HTTP 500 also logs the handler stack; the response body stays generic.
func writeJSONError(w http.ResponseWriter, status int, code string) {
	writeJSONErrorCause(w, status, code, nil)
}

// writeJSONErrorCause is writeJSONError plus an optional cause logged only
// for HTTP 500. The cause never appears in the response body.
func writeJSONErrorCause(w http.ResponseWriter, status int, code string, err error) {
	if status == http.StatusInternalServerError {
		logHTTP500(w, status, code, err)
	}
	writeJSONErrorBody(w, status, code)
}

func writeJSONErrorBody(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func logHTTP500(w http.ResponseWriter, status int, code string, err error) {
	log, r := loggerAndRequest(w)
	if log == nil {
		return
	}
	fields := logrus.Fields{
		"status": status,
		"code":   code,
	}
	if r != nil {
		fields["method"] = r.Method
		fields["path"] = r.URL.Path
		fields["request_id"] = chimiddleware.GetReqID(r.Context())
	}
	fields["stack"] = string(debug.Stack())
	entry := log.WithFields(fields)
	if err != nil {
		entry = entry.WithError(err)
	}
	entry.Error("http 500")
}

func logHTTPPanic(w http.ResponseWriter, r *http.Request, rec any) {
	log, _ := loggerAndRequest(w)
	if log == nil && r != nil {
		if l, ok := r.Context().Value(requestLogCtxKey{}).(*logrus.Logger); ok {
			log = l
		}
	}
	if log == nil {
		return
	}
	fields := logrus.Fields{"panic": rec}
	if r != nil {
		fields["method"] = r.Method
		fields["path"] = r.URL.Path
		fields["request_id"] = chimiddleware.GetReqID(r.Context())
	}
	fields["stack"] = string(debug.Stack())
	log.WithFields(fields).Error("http panic")
}

func loggerAndRequest(w http.ResponseWriter) (*logrus.Logger, *http.Request) {
	for cur := w; cur != nil; {
		if lw, ok := cur.(*responseLogWriter); ok {
			return lw.log, lw.r
		}
		u, ok := cur.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return nil, nil
		}
		cur = u.Unwrap()
	}
	return nil, nil
}
