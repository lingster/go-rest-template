package routers

import (
	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/server/web/context/param"
)

func init() {

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"],
        beego.ControllerComments{
            Method: "AuditLogs",
            Router: `/audit-logs`,
            AllowHTTPMethods: []string{"get"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"],
        beego.ControllerComments{
            Method: "Stats",
            Router: `/stats`,
            AllowHTTPMethods: []string{"get"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"],
        beego.ControllerComments{
            Method: "ListUsers",
            Router: `/users`,
            AllowHTTPMethods: []string{"get"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"],
        beego.ControllerComments{
            Method: "GetUser",
            Router: `/users/:id`,
            AllowHTTPMethods: []string{"get"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"],
        beego.ControllerComments{
            Method: "UpdateUser",
            Router: `/users/:id`,
            AllowHTTPMethods: []string{"patch"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"],
        beego.ControllerComments{
            Method: "DeleteUser",
            Router: `/users/:id`,
            AllowHTTPMethods: []string{"delete"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AdminController"],
        beego.ControllerComments{
            Method: "RevokeSessions",
            Router: `/users/:id/revoke-sessions`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"],
        beego.ControllerComments{
            Method: "ForgotPassword",
            Router: `/forgot-password`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"],
        beego.ControllerComments{
            Method: "LoginGoogle",
            Router: `/google`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"],
        beego.ControllerComments{
            Method: "Login",
            Router: `/login`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"],
        beego.ControllerComments{
            Method: "LoginMFA",
            Router: `/login/2fa`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"],
        beego.ControllerComments{
            Method: "Logout",
            Router: `/logout`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"],
        beego.ControllerComments{
            Method: "PasskeyLoginBegin",
            Router: `/passkey/login/begin`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"],
        beego.ControllerComments{
            Method: "PasskeyLoginFinish",
            Router: `/passkey/login/finish`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"],
        beego.ControllerComments{
            Method: "Providers",
            Router: `/providers`,
            AllowHTTPMethods: []string{"get"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"],
        beego.ControllerComments{
            Method: "Refresh",
            Router: `/refresh`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"],
        beego.ControllerComments{
            Method: "Register",
            Router: `/register`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"],
        beego.ControllerComments{
            Method: "ResendVerification",
            Router: `/resend-verification`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"],
        beego.ControllerComments{
            Method: "ResetPassword",
            Router: `/reset-password`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:AuthController"],
        beego.ControllerComments{
            Method: "VerifyEmail",
            Router: `/verify-email`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"],
        beego.ControllerComments{
            Method: "Profile",
            Router: `/`,
            AllowHTTPMethods: []string{"get"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"],
        beego.ControllerComments{
            Method: "UpdateProfile",
            Router: `/`,
            AllowHTTPMethods: []string{"patch"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"],
        beego.ControllerComments{
            Method: "RecoveryCodes",
            Router: `/2fa/recovery-codes`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"],
        beego.ControllerComments{
            Method: "TOTPDisable",
            Router: `/2fa/totp/disable`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"],
        beego.ControllerComments{
            Method: "TOTPEnable",
            Router: `/2fa/totp/enable`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"],
        beego.ControllerComments{
            Method: "TOTPSetup",
            Router: `/2fa/totp/setup`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"],
        beego.ControllerComments{
            Method: "ChangePassword",
            Router: `/change-password`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"],
        beego.ControllerComments{
            Method: "ListPasskeys",
            Router: `/passkeys`,
            AllowHTTPMethods: []string{"get"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"],
        beego.ControllerComments{
            Method: "DeletePasskey",
            Router: `/passkeys/:id`,
            AllowHTTPMethods: []string{"delete"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"],
        beego.ControllerComments{
            Method: "PasskeyRegisterBegin",
            Router: `/passkeys/register/begin`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

    beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"] = append(beego.GlobalControllerRouter["github.com/lingster/go-rest-template/backend/controllers:MeController"],
        beego.ControllerComments{
            Method: "PasskeyRegisterFinish",
            Router: `/passkeys/register/finish`,
            AllowHTTPMethods: []string{"post"},
            MethodParams: param.Make(),
            Filters: nil,
            Params: nil})

}
