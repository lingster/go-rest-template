// Package controllers contains thin Beego controllers; all logic lives in services.
package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/beego/beego/v2/server/web"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/container"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/filters"
	"github.com/lingster/go-rest-template/backend/httpx"
	"github.com/lingster/go-rest-template/backend/models"
	"github.com/lingster/go-rest-template/backend/services"
)

const maxBodyBytes = 1 << 20

// BaseController provides JSON/error helpers and access to the service container.
type BaseController struct {
	web.Controller
	C *container.Container
}

// Prepare runs before every action.
func (b *BaseController) Prepare() { b.C = container.Get() }

// Reply writes v as JSON with the given status.
func (b *BaseController) Reply(status int, v interface{}) {
	httpx.WriteJSON(b.Ctx.ResponseWriter, status, v)
}

// OK writes a 200 JSON response.
func (b *BaseController) OK(v interface{}) { b.Reply(http.StatusOK, v) }

// Message writes {"message": msg} with the given status.
func (b *BaseController) Message(status int, msg string) { b.Reply(status, dto.Message{Message: msg}) }

// NoContent writes an empty 204.
func (b *BaseController) NoContent() {
	b.Ctx.ResponseWriter.WriteHeader(http.StatusNoContent)
}

// Fail writes the error envelope for err.
func (b *BaseController) Fail(err error) { httpx.WriteError(b.Ctx.ResponseWriter, err) }

// Decode parses and validates the JSON body into dst; on failure it writes the error and returns false.
func (b *BaseController) Decode(dst dto.Validator) bool { return b.decode(dst, false) }

// DecodeOptional is like Decode but accepts an empty body.
func (b *BaseController) DecodeOptional(dst dto.Validator) bool { return b.decode(dst, true) }

func (b *BaseController) decode(dst dto.Validator, optional bool) bool {
	body := http.MaxBytesReader(b.Ctx.ResponseWriter, b.Ctx.Request.Body, maxBodyBytes)
	err := json.NewDecoder(body).Decode(dst)
	switch {
	case err == nil:
	case errors.Is(err, io.EOF) && optional:
	case errors.Is(err, io.EOF):
		b.Fail(apperr.Validation(map[string]string{"body": "request body is required"}))
		return false
	default:
		b.Fail(apperr.Validation(map[string]string{"body": "invalid JSON"}))
		return false
	}
	if d := dst.Validate(); len(d) > 0 {
		b.Fail(apperr.Validation(d))
		return false
	}
	return true
}

// ReqContext returns the request context enriched with IP / user agent for audit logging.
func (b *BaseController) ReqContext() context.Context {
	r := b.Ctx.Request
	return services.WithMeta(r.Context(), services.RequestMeta{
		IP: httpx.ClientIP(r, b.C.Cfg.TrustProxy), UserAgent: r.UserAgent(),
	})
}

// CurrentUser returns the user put in place by filters.Authenticate.
func (b *BaseController) CurrentUser() *models.User {
	u, _ := b.Ctx.Input.GetData(filters.CurrentUserKey).(*models.User)
	return u
}

// jsonBytes re-encodes a decoded JSON value (used for WebAuthn credential payloads).
func jsonBytes(v interface{}) ([]byte, error) { return json.Marshal(v) }

// loginReply writes either the full session or the MFA challenge.
func (b *BaseController) loginReply(res *services.LoginResult, err error) {
	if err != nil {
		b.Fail(err)
		return
	}
	if res.Challenge != nil {
		b.OK(res.Challenge)
		return
	}
	b.OK(res.Auth)
}
