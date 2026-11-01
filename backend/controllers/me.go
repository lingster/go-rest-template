package controllers

import (
	"net/http"

	"github.com/lingster/go-rest-template/backend/dto"
)

// MeController handles the authenticated user's own account.
type MeController struct{ BaseController }

// Profile returns the current user
// @Title Profile
// @Security BearerAuth
// @Success 200 {object} dto.User
// @Failure 401 unauthorized
// @router / [get]
func (c *MeController) Profile() {
	v, err := c.C.Users.Present(c.CurrentUser())
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(v)
}

// UpdateProfile changes the display name
// @Title UpdateProfile
// @Security BearerAuth
// @Param	body	body	dto.UpdateProfileRequest	true	"New name"
// @Success 200 {object} dto.User
// @router / [patch]
func (c *MeController) UpdateProfile() {
	var req dto.UpdateProfileRequest
	if !c.Decode(&req) {
		return
	}
	u := c.CurrentUser()
	if err := c.C.Users.UpdateName(c.ReqContext(), u, req.Name); err != nil {
		c.Fail(err)
		return
	}
	c.Profile()
}

// ChangePassword changes the password
// @Title ChangePassword
// @Security BearerAuth
// @Param	body	body	dto.ChangePasswordRequest	true	"Current and new password"
// @Success 200 {object} dto.Message
// @Failure 401 invalid_credentials
// @router /change-password [post]
func (c *MeController) ChangePassword() {
	var req dto.ChangePasswordRequest
	if !c.Decode(&req) {
		return
	}
	if err := c.C.Auth.ChangePassword(c.ReqContext(), c.CurrentUser(), req.CurrentPassword, req.NewPassword); err != nil {
		c.Fail(err)
		return
	}
	c.Message(http.StatusOK, "Password changed")
}

// TOTPSetup generates a pending TOTP secret
// @Title TOTPSetup
// @Security BearerAuth
// @Success 200 {object} dto.TOTPSetupResponse
// @router /2fa/totp/setup [post]
func (c *MeController) TOTPSetup() {
	secret, url, err := c.C.MFA.Setup(c.CurrentUser())
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(dto.TOTPSetupResponse{Secret: secret, OTPAuthURL: url})
}

// TOTPEnable activates TOTP
// @Title TOTPEnable
// @Security BearerAuth
// @Param	body	body	dto.CodeRequest	true	"Current TOTP code"
// @Success 200 {object} dto.RecoveryCodesResponse
// @Failure 401 invalid_mfa_code
// @router /2fa/totp/enable [post]
func (c *MeController) TOTPEnable() {
	var req dto.CodeRequest
	if !c.Decode(&req) {
		return
	}
	codes, err := c.C.MFA.Enable(c.ReqContext(), c.CurrentUser(), req.Code)
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(dto.RecoveryCodesResponse{RecoveryCodes: codes})
}

// TOTPDisable turns TOTP off
// @Title TOTPDisable
// @Security BearerAuth
// @Param	body	body	dto.DisableTOTPRequest	true	"Password (omit for password-less accounts) and TOTP/recovery code"
// @Success 200 {object} dto.Message
// @router /2fa/totp/disable [post]
func (c *MeController) TOTPDisable() {
	var req dto.DisableTOTPRequest
	if !c.Decode(&req) {
		return
	}
	if err := c.C.MFA.Disable(c.ReqContext(), c.CurrentUser(), req.Password, req.Code); err != nil {
		c.Fail(err)
		return
	}
	c.Message(http.StatusOK, "Two-factor authentication disabled")
}

// RecoveryCodes regenerates recovery codes
// @Title RegenerateRecoveryCodes
// @Security BearerAuth
// @Param	body	body	dto.CodeRequest	true	"TOTP or recovery code"
// @Success 200 {object} dto.RecoveryCodesResponse
// @router /2fa/recovery-codes [post]
func (c *MeController) RecoveryCodes() {
	var req dto.CodeRequest
	if !c.Decode(&req) {
		return
	}
	codes, err := c.C.MFA.RegenerateRecoveryCodes(c.ReqContext(), c.CurrentUser(), req.Code)
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(dto.RecoveryCodesResponse{RecoveryCodes: codes})
}

// ListPasskeys lists registered passkeys
// @Title ListPasskeys
// @Security BearerAuth
// @Success 200 {array} dto.Passkey
// @router /passkeys [get]
func (c *MeController) ListPasskeys() {
	list, err := c.C.Passkeys.List(c.CurrentUser().ID)
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(list)
}

// PasskeyRegisterBegin starts passkey registration
// @Title PasskeyRegisterBegin
// @Security BearerAuth
// @Success 200 {object} dto.CeremonyResponse
// @router /passkeys/register/begin [post]
func (c *MeController) PasskeyRegisterBegin() {
	sid, opts, err := c.C.Passkeys.BeginRegistration(c.CurrentUser())
	if err != nil {
		c.Fail(err)
		return
	}
	c.OK(dto.CeremonyResponse{SessionID: sid, Options: opts})
}

// PasskeyRegisterFinish completes passkey registration
// @Title PasskeyRegisterFinish
// @Security BearerAuth
// @Param	body	body	dto.PasskeyRegisterFinishRequest	true	"Session, name and credential"
// @Success 201 {object} dto.Passkey
// @router /passkeys/register/finish [post]
func (c *MeController) PasskeyRegisterFinish() {
	var req dto.PasskeyRegisterFinishRequest
	if !c.Decode(&req) {
		return
	}
	raw, err := jsonBytes(req.Credential)
	if err != nil {
		c.Fail(err)
		return
	}
	p, err := c.C.Passkeys.FinishRegistration(c.ReqContext(), c.CurrentUser(), req.SessionID, req.Name, raw)
	if err != nil {
		c.Fail(err)
		return
	}
	c.Reply(http.StatusCreated, p)
}

// DeletePasskey removes a passkey
// @Title DeletePasskey
// @Security BearerAuth
// @Param	id	path	string	true	"Passkey id"
// @Success 204 no content
// @Failure 404 not_found
// @router /passkeys/:id [delete]
func (c *MeController) DeletePasskey() {
	if err := c.C.Passkeys.Delete(c.ReqContext(), c.CurrentUser().ID, c.Ctx.Input.Param(":id")); err != nil {
		c.Fail(err)
		return
	}
	c.NoContent()
}
