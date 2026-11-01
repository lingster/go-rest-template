package controllers

import (
	"net/http"

	"github.com/lingster/go-rest-template/backend/dto"
)

// AuthController handles public authentication endpoints.
type AuthController struct{ BaseController }

// Register creates an account
// @Title Register
// @Description Create a new account. Sends a verification email.
// @Param	body	body	dto.RegisterRequest	true	"Account details"
// @Success 201 {object} dto.RegisterResponse
// @Failure 409 conflict: email already registered
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
	c.Reply(http.StatusCreated, res)
}

// Login authenticates with email and password
// @Title Login
// @Description Returns an AuthResponse, or an MfaChallenge (mfa_required=true) when 2FA is enabled.
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

// LoginMFA completes a 2FA login
// @Title LoginMFA
// @Description Exchange the mfa_token plus a TOTP or recovery code for a session.
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
// @Description Rotating refresh; re-using a revoked token revokes the whole token family.
// @Param	body	body	dto.RefreshRequest	true	"Refresh token"
// @Success 200 {object} dto.Tokens
// @Failure 401 invalid_token or token_expired
// @router /refresh [post]
func (c *AuthController) Refresh() {
	var req dto.RefreshRequest
	if !c.Decode(&req) {
		return
	}
	tokens, err := c.C.Auth.Refresh(c.ReqContext(), req.RefreshToken)
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(tokens)
}

// Logout revokes a refresh token
// @Title Logout
// @Param	body	body	dto.RefreshRequest	true	"Refresh token to revoke"
// @Success 204 no content
// @router /logout [post]
func (c *AuthController) Logout() {
	var req dto.RefreshRequest
	if !c.Decode(&req) {
		return
	}
	if err := c.C.Auth.Logout(c.ReqContext(), req.RefreshToken); err != nil {
		c.Fail(err)
		return
	}
	c.NoContent()
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
// @Description Omit email to use discoverable credentials.
// @Param	body	body	dto.PasskeyLoginBeginRequest	false	"Optional email"
// @Success 200 {object} dto.CeremonyResponse
// @router /passkey/login/begin [post]
func (c *AuthController) PasskeyLoginBegin() {
	var req dto.PasskeyLoginBeginRequest
	if !c.DecodeOptional(&req) {
		return
	}
	sid, opts, err := c.C.Passkeys.BeginLogin(dto.NormalizeEmail(req.Email))
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(dto.CeremonyResponse{SessionID: sid, Options: opts})
}

// PasskeyLoginFinish completes a passkey login ceremony
// @Title PasskeyLoginFinish
// @Description A successful passkey login satisfies MFA.
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
