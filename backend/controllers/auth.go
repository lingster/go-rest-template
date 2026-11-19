package controllers

import (
	"net/http"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/dto"
	"github.com/lingster/go-rest-template/backend/httpx"
)

// AuthController handles public authentication endpoints.
type AuthController struct{ BaseController }

// Register creates an account
// @Title Register
// @Description Create a new account and send a verification email. The response is the same whether or not the address is already registered (an existing owner is emailed instead), so it cannot be used to enumerate accounts.
// @Param	body	body	dto.RegisterRequest	true	"Account details"
// @Success 202 {object} dto.RegisterResponse
// @Failure 422 validation_error
// @Failure 429 rate_limited
// @router /register [post]
func (c *AuthController) Register() {
	var req dto.RegisterRequest
	if !c.Decode(&req) {
		return
	}
	res, err := c.C.Auth.Register(c.ReqContext(), &req)
	if err != nil {
		c.Fail(err)
		return
	}
	c.Reply(http.StatusAccepted, res)
}

// Login authenticates with email and password
// @Title Login
// @Description Returns an AuthResponse, or an MfaChallenge (mfa_required=true) when 2FA is enabled.
// @Param	X-Session-Transport	header	string	false	"cookie: set the refresh token as an HttpOnly cookie and omit it from the body"
// @Param	body	body	dto.LoginRequest	true	"Credentials"
// @Success 200 {object} dto.AuthResponse
// @Failure 401 invalid_credentials
// @Failure 403 email_not_verified or account_disabled
// @Failure 429 rate_limited
// @router /login [post]
func (c *AuthController) Login() {
	var req dto.LoginRequest
	if !c.Decode(&req) {
		return
	}
	res, err := c.C.Auth.Login(c.ReqContext(), &req)
	c.loginReply(res, err)
}

// Providers lists enabled external sign-in providers
// @Title Providers
// @Description Public configuration so clients know whether to show "Continue with Google". web_client_id is not a secret.
// @Success 200 {object} dto.ProvidersResponse
// @router /providers [get]
func (c *AuthController) Providers() {
	c.OK(c.C.Auth.Providers())
}

// LoginGoogle signs in with a Google ID token
// @Title LoginGoogle
// @Description Verifies a Google ID token server-side. Creates the account, or links it to an existing account with the same email only when Google is authoritative for that address (gmail.com/googlemail.com, or the token's hosted domain claim equals the email domain); otherwise 409 account_exists_sign_in_required. Returns an AuthResponse, or an MfaChallenge (mfa_required=true) when TOTP is enabled: Google never bypasses 2FA. Disabled (404 google_signin_disabled) unless GOOGLE_CLIENT_IDS is set.
// @Param	X-Session-Transport	header	string	false	"cookie: set the refresh token as an HttpOnly cookie and omit it from the body"
// @Param	body	body	dto.GoogleLoginRequest	true	"Google ID token"
// @Success 200 {object} dto.AuthResponse
// @Failure 401 invalid_token
// @Failure 403 account_disabled
// @Failure 404 google_signin_disabled
// @Failure 409 account_exists_sign_in_required
// @Failure 429 rate_limited
// @router /google [post]
func (c *AuthController) LoginGoogle() {
	var req dto.GoogleLoginRequest
	if !c.Decode(&req) {
		return
	}
	res, err := c.C.Auth.LoginGoogle(c.ReqContext(), req.IDToken)
	c.loginReply(res, err)
}

// LoginMFA completes a 2FA login
// @Title LoginMFA
// @Description Exchange the mfa_token plus a TOTP or recovery code for a session.
// @Param	X-Session-Transport	header	string	false	"cookie: set the refresh token as an HttpOnly cookie and omit it from the body"
// @Param	body	body	dto.MFALoginRequest	true	"MFA token and code"
// @Success 200 {object} dto.AuthResponse
// @Failure 401 invalid_mfa_code, invalid_token or token_expired
// @router /login/2fa [post]
func (c *AuthController) LoginMFA() {
	var req dto.MFALoginRequest
	if !c.Decode(&req) {
		return
	}
	res, err := c.C.Auth.LoginMFA(c.ReqContext(), &req)
	c.loginReply(res, err)
}

// Refresh rotates the refresh token
// @Title Refresh
// @Description Rotating refresh; re-using a revoked token revokes the whole token family. Webapp clients send X-Session-Transport: cookie and no body: the token is read from, and the rotated one written to, the HttpOnly refresh cookie (a failed refresh clears it).
// @Param	X-Session-Transport	header	string	false	"cookie: refresh token travels in the HttpOnly cookie instead of the body"
// @Param	body	body	dto.RefreshRequest	false	"Refresh token (omit when using the cookie)"
// @Success 200 {object} dto.Tokens
// @Failure 401 invalid_token or token_expired
// @Failure 403 forbidden: cookie request without X-Session-Transport or from a disallowed Origin
// @Failure 422 validation_error: no refresh token in the body or cookie
// @router /refresh [post]
func (c *AuthController) Refresh() {
	token, fromCookie, ok := c.presentedRefreshToken()
	if !ok {
		return
	}
	tokens, err := c.C.Auth.Refresh(c.ReqContext(), token)
	if err != nil {
		if fromCookie && apperr.As(err).Status < http.StatusInternalServerError {
			c.refreshCookie().Clear(c.Ctx.ResponseWriter)
		}
		c.Fail(err)
		return
	}
	c.deliverRefreshToken(tokens)
	c.OK(tokens)
}

// Logout revokes a refresh token
// @Title Logout
// @Description Idempotent. With X-Session-Transport: cookie the refresh cookie is revoked (when no body token is given) and cleared.
// @Param	X-Session-Transport	header	string	false	"cookie: revoke and clear the HttpOnly refresh cookie"
// @Param	body	body	dto.RefreshRequest	false	"Refresh token to revoke (omit when using the cookie)"
// @Success 204 no content
// @Failure 403 forbidden: cookie request without X-Session-Transport or from a disallowed Origin
// @Failure 422 validation_error: no refresh token in the body or cookie
// @router /logout [post]
func (c *AuthController) Logout() {
	token, _, ok := c.presentedRefreshToken()
	if !ok {
		return
	}
	if err := c.C.Auth.Logout(c.ReqContext(), token); err != nil {
		c.Fail(err)
		return
	}
	if httpx.WantsCookieSession(c.Ctx.Request) {
		c.refreshCookie().Clear(c.Ctx.ResponseWriter)
	}
	c.NoContent()
}

// refreshBody decodes an optional refresh token; it is validated after the cookie fallback.
type refreshBody struct{ dto.RefreshRequest }

func (refreshBody) Validate() map[string]string { return nil }

// presentedRefreshToken returns the refresh token from the JSON body or, failing that, from the
// refresh cookie after the CSRF check. A cookie-transport client without a cookie gets "" (no session),
// which the services reject (refresh) or ignore (logout). ok=false means an error was written.
func (c *AuthController) presentedRefreshToken() (token string, fromCookie, ok bool) {
	var body refreshBody
	if !c.DecodeOptional(&body) {
		return "", false, false
	}
	problems := body.RefreshRequest.Validate()
	if len(problems) == 0 {
		return body.RefreshToken, false, true
	}
	r := c.Ctx.Request
	if raw := c.refreshCookie().Read(r); raw != "" {
		if err := httpx.CheckCookieRequest(r, httpx.NewOrigins(c.C.Cfg.CORSAllowedOrigins)); err != nil {
			c.Fail(err)
			return "", false, false
		}
		return raw, true, true
	}
	if httpx.WantsCookieSession(r) {
		// Nothing to clear: a Set-Cookie here could wipe the cookie of a concurrent login.
		return "", false, true
	}
	c.Fail(apperr.Validation(problems))
	return "", false, false
}

// VerifyEmail confirms an email address
// @Title VerifyEmail
// @Param	body	body	dto.TokenRequest	true	"Verification token from the email link"
// @Success 200 {object} dto.Message
// @Failure 400 invalid_token
// @router /verify-email [post]
func (c *AuthController) VerifyEmail() {
	var req dto.TokenRequest
	if !c.Decode(&req) {
		return
	}
	if err := c.C.Auth.VerifyEmail(c.ReqContext(), req.Token); err != nil {
		c.Fail(err)
		return
	}
	c.Message(http.StatusOK, "Email verified")
}

// ResendVerification re-sends the verification email
// @Title ResendVerification
// @Description Always returns 202 to avoid leaking which emails exist.
// @Param	body	body	dto.EmailRequest	true	"Email"
// @Success 202 {object} dto.Message
// @router /resend-verification [post]
func (c *AuthController) ResendVerification() {
	var req dto.EmailRequest
	if !c.Decode(&req) {
		return
	}
	c.C.Auth.ResendVerification(c.ReqContext(), req.Email)
	c.Message(http.StatusAccepted, "If the account exists and is unverified, a verification email has been sent")
}

// ForgotPassword starts a password reset
// @Title ForgotPassword
// @Description Always returns 202 (no user enumeration).
// @Param	body	body	dto.EmailRequest	true	"Email"
// @Success 202 {object} dto.Message
// @router /forgot-password [post]
func (c *AuthController) ForgotPassword() {
	var req dto.EmailRequest
	if !c.Decode(&req) {
		return
	}
	c.C.Auth.ForgotPassword(c.ReqContext(), req.Email)
	c.Message(http.StatusAccepted, "If the account exists, a password reset email has been sent")
}

// ResetPassword completes a password reset
// @Title ResetPassword
// @Description Sets a new password and revokes all refresh tokens of the user.
// @Param	body	body	dto.ResetPasswordRequest	true	"Reset token and new password"
// @Success 200 {object} dto.Message
// @Failure 400 invalid_token
// @Failure 422 validation_error
// @router /reset-password [post]
func (c *AuthController) ResetPassword() {
	var req dto.ResetPasswordRequest
	if !c.Decode(&req) {
		return
	}
	if err := c.C.Auth.ResetPassword(c.ReqContext(), req.Token, req.Password); err != nil {
		c.Fail(err)
		return
	}
	c.Message(http.StatusOK, "Password updated")
}

// PasskeyLoginBegin starts a passkey login ceremony
// @Title PasskeyLoginBegin
// @Description Always a discoverable-credential ceremony with user verification required. The optional email is accepted for compatibility and ignored, so the response never depends on whether an account exists.
// @Param	body	body	dto.PasskeyLoginBeginRequest	false	"Optional email (ignored)"
// @Success 200 {object} dto.CeremonyResponse
// @router /passkey/login/begin [post]
func (c *AuthController) PasskeyLoginBegin() {
	var req dto.PasskeyLoginBeginRequest
	if !c.DecodeOptional(&req) {
		return
	}
	sid, opts, err := c.C.Passkeys.BeginLogin()
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(dto.CeremonyResponse{SessionID: sid, Options: opts})
}

// PasskeyLoginFinish completes a passkey login ceremony
// @Title PasskeyLoginFinish
// @Description A successful passkey login satisfies MFA.
// @Param	X-Session-Transport	header	string	false	"cookie: set the refresh token as an HttpOnly cookie and omit it from the body"
// @Param	body	body	dto.PasskeyFinishRequest	true	"Session and credential"
// @Success 200 {object} dto.AuthResponse
// @Failure 401 invalid_credentials
// @router /passkey/login/finish [post]
func (c *AuthController) PasskeyLoginFinish() {
	var req dto.PasskeyFinishRequest
	if !c.Decode(&req) {
		return
	}
	raw, err := jsonBytes(req.Credential)
	if err != nil {
		c.Fail(err)
		return
	}
	res, err := c.C.Auth.LoginPasskey(c.ReqContext(), req.SessionID, raw)
	c.loginReply(res, err)
}
