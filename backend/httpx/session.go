package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/lingster/go-rest-template/backend/apperr"
)

// Browser clients send "X-Session-Transport: cookie" to receive the refresh token in an HttpOnly
// cookie instead of the JSON body. Native clients omit it and keep body tokens.
const (
	SessionTransportHeader = "X-Session-Transport"
	SessionTransportCookie = "cookie"
)

// RefreshCookiePath scopes the refresh cookie to the auth endpoints (refresh, logout).
const RefreshCookiePath = "/api/v1/auth"

// WantsCookieSession reports whether the client opted into the cookie session transport.
func WantsCookieSession(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get(SessionTransportHeader)), SessionTransportCookie)
}

// Origins is a browser-origin allow-list (scheme://host[:port], trailing slash ignored); "*" allows any origin.
type Origins map[string]bool

// NewOrigins builds an allow-list from configuration values.
func NewOrigins(list []string) Origins {
	o := make(Origins, len(list))
	for _, v := range list {
		o[strings.TrimRight(v, "/")] = true
	}
	return o
}

// Wildcard reports whether "*" is configured.
func (o Origins) Wildcard() bool { return o["*"] }

// Explicit reports whether origin is listed by name ("*" never counts): only such origins may make
// credentialed (cookie) requests.
func (o Origins) Explicit(origin string) bool { return origin != "" && origin != "*" && o[origin] }

// CheckCookieRequest is the CSRF guard for requests authenticated by the refresh cookie. The request
// must carry the session transport header, which a cross-site form or simple request cannot send
// without passing a CORS preflight, and an Origin header, when present, must be explicitly allowed.
func CheckCookieRequest(r *http.Request, allowed Origins) error {
	if !WantsCookieSession(r) {
		return apperr.Forbidden("Cookie sessions require the " + SessionTransportHeader + " header")
	}
	if origin := r.Header.Get("Origin"); origin != "" && !allowed.Explicit(origin) {
		return apperr.Forbidden("Origin not allowed")
	}
	return nil
}

// RefreshCookie writes, reads and clears the HttpOnly refresh-token cookie.
type RefreshCookie struct {
	// Secure adds the Secure attribute and the __Secure- name prefix (off only for plain-HTTP dev).
	Secure bool
	// Domain is empty for a host-only cookie, or a parent domain shared by the webapp and API hosts.
	Domain string
	// TTL is the refresh token lifetime (cookie Max-Age).
	TTL time.Duration
}

// Name is "__Secure-rt" when Secure (the __Host- prefix would forbid the path), else "rt".
func (c RefreshCookie) Name() string {
	if c.Secure {
		return "__Secure-rt"
	}
	return "rt"
}

// Set stores token in the cookie.
func (c RefreshCookie) Set(w http.ResponseWriter, token string) {
	http.SetCookie(w, c.cookie(token, int(c.TTL/time.Second)))
}

// Clear expires the cookie (same attributes, Max-Age=0).
func (c RefreshCookie) Clear(w http.ResponseWriter) { http.SetCookie(w, c.cookie("", -1)) }

// Read returns the cookie value, or "" when absent.
func (c RefreshCookie) Read(r *http.Request) string {
	ck, err := r.Cookie(c.Name())
	if err != nil {
		return ""
	}
	return ck.Value
}

func (c RefreshCookie) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name: c.Name(), Value: value, Path: RefreshCookiePath, Domain: c.Domain, MaxAge: maxAge,
		Secure: c.Secure, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	}
}
