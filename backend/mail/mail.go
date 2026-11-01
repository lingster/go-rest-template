// Package mail provides the Mailer abstraction with console and SMTP implementations.
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	htmltemplate "html/template"
	"log"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	texttemplate "text/template"
	"time"
)

// Message is an outgoing email.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Mailer sends emails.
type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// ---------- console ----------

// ConsoleMailer logs emails (including links) instead of sending them; for development.
type ConsoleMailer struct{}

func (ConsoleMailer) Send(_ context.Context, m Message) error {
	log.Printf("[mail:console] to=%s subject=%q\n----\n%s\n----", m.To, m.Subject, m.Text)
	return nil
}

// ---------- smtp ----------

// SMTPMailer sends multipart/alternative emails through an SMTP server.
// STARTTLS is used automatically when the server advertises it; auth only when a username is set.
type SMTPMailer struct {
	Host, Username, Password, From string
	Port                           int
}

func (s *SMTPMailer) Send(_ context.Context, m Message) error {
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("invalid MAIL_FROM: %w", err)
	}
	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}
	return smtp.SendMail(addr, auth, from.Address, []string{m.To}, buildMIME(s.From, m))
}

func sanitizeHeader(s string) string { return strings.NewReplacer("\r", " ", "\n", " ").Replace(s) }

func buildMIME(from string, m Message) []byte {
	var boundary [12]byte
	_, _ = rand.Read(boundary[:])
	b := hex.EncodeToString(boundary[:])
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\n",
		sanitizeHeader(from), sanitizeHeader(m.To), sanitizeHeader(mimeSubject(m.Subject)), time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", b)
	for _, part := range []struct{ ct, body string }{{"text/plain", m.Text}, {"text/html", m.HTML}} {
		fmt.Fprintf(&buf, "--%s\r\nContent-Type: %s; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n", b, part.ct, part.body)
	}
	fmt.Fprintf(&buf, "--%s--\r\n", b)
	return buf.Bytes()
}

func mimeSubject(s string) string {
	for _, r := range s {
		if r > 127 {
			return "=?UTF-8?q?" + strings.ReplaceAll(s, " ", "_") + "?="
		}
	}
	return s
}

// ---------- templates ----------

type tmplData struct{ Name, Link, App string }

type tmplPair struct {
	subject string
	text    *texttemplate.Template
	html    *htmltemplate.Template
}

const layoutHTML = `<!doctype html><html><body style="font-family:sans-serif;max-width:560px;margin:auto">
<h2>%s</h2><p>Hi {{.Name}},</p><p>%s</p>
<p><a href="{{.Link}}">%s</a></p>
<p>Or copy this link into your browser:<br>{{.Link}}</p>
<p style="color:#666">%s</p></body></html>`

func newPair(subject, intro, button, outro string) tmplPair {
	text := fmt.Sprintf("Hi {{.Name}},\n\n%s\n\n{{.Link}}\n\n%s\n", intro, outro)
	return tmplPair{
		subject: subject,
		text:    texttemplate.Must(texttemplate.New("t").Parse(text)),
		html:    htmltemplate.Must(htmltemplate.New("h").Parse(fmt.Sprintf(layoutHTML, subject, intro, button, outro))),
	}
}

var (
	verifyTmpl = newPair("Verify your email address", "Please confirm your email address to activate your account.",
		"Verify email", "This link expires in 24 hours. If you did not create an account you can ignore this email.")
	resetTmpl = newPair("Reset your password", "We received a request to reset your password.",
		"Reset password", "This link expires in 1 hour. If you did not request this you can ignore this email.")
)

func render(p tmplPair, to, name, link string) Message {
	d := tmplData{Name: name, Link: link}
	var t, h bytes.Buffer
	_ = p.text.Execute(&t, d)
	_ = p.html.Execute(&h, d)
	return Message{To: to, Subject: p.subject, Text: t.String(), HTML: h.String()}
}

// VerifyEmail builds the verification email.
func VerifyEmail(to, name, link string) Message { return render(verifyTmpl, to, name, link) }

// ResetPassword builds the password reset email.
func ResetPassword(to, name, link string) Message { return render(resetTmpl, to, name, link) }
