// Package httpx has small net/http helpers shared by filters and controllers.
package httpx

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/dto"
)

// ClientIP returns the caller's IP. X-Forwarded-For is honoured only when trustProxy is set.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if v != nil {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			log.Printf("httpx: encode response: %v", err)
		}
	}
}

// WriteError writes the contract error envelope for err (unknown errors become 500 internal_error).
func WriteError(w http.ResponseWriter, err error) {
	ae := apperr.As(err)
	if ae.Code == apperr.CodeInternal {
		log.Printf("internal error: %v", err)
	}
	if ae.Status == http.StatusTooManyRequests {
		w.Header().Set("Retry-After", "60")
	}
	WriteJSON(w, ae.Status, dto.ErrorResponse{Error: dto.ErrorBody{Code: ae.Code, Message: ae.Message, Details: ae.Details}})
}
