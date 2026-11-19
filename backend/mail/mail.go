// Package mail provides the Mailer abstraction with console and SMTP implementations.
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
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

// smtpTimeout bounds a whole SMTP conversation so a slow or hung server cannot pin goroutines.
const smtpTimeout = 30 * time.Second

// SMTPMailer sends multipart/alternative emails through an SMTP server.
// STARTTLS is used whenever the server advertises it; with RequireTLS a server that does not (or an
// attacker stripping the STARTTLS capability) makes the send fail instead of falling back to
// plaintext. Auth is used only when a username is set.
type SMTPMailer struct {
	Host, Username, Password, From string
	Port                           int
	RequireTLS                     bool
}

func (s *SMTPMailer) Send(ctx context.Context, m Message) error {
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("invalid MAIL_FROM: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, smtpTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(s.Host, strconv.Itoa(s.Port)))
	if err != nil {
		return err
	}
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if err := s.secure(c); err != nil {
		return err
	}
	if s.Username != "" {
		// PlainAuth itself refuses to send credentials over an unencrypted non-localhost connection.
		if err := c.Auth(smtp.PlainAuth("", s.Username, s.Password, s.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(m.To); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(buildMIME(s.From, m)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func (s *SMTPMailer) secure(c *smtp.Client) error {
	if ok, _ := c.Extension("STARTTLS"); ok {
		return c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12})
	}
	if s.RequireTLS {
		return fmt.Errorf("smtp server %s does not offer STARTTLS and SMTP_REQUIRE_TLS is set", s.Host)
	}
	return nil
}

// ---------- async ----------

// AsyncMailer hands messages to a small worker pool so callers never wait on the mail server. This
// keeps response times of the no-enumeration endpoints (forgot password, resend verification) the
// same whether or not a mail is sent, and keeps a slow SMTP server from stalling requests. The queue
// is bounded: when it is full the message is dropped and logged rather than blocking the caller.
type AsyncMailer struct {
	next  Mailer
	queue chan Message
}

// NewAsync starts workers sending through next.
func NewAsync(next Mailer, workers, queueSize int) *AsyncMailer {
	a := &AsyncMailer{next: next, queue: make(chan Message, queueSize)}
	for i := 0; i < workers; i++ {
		go a.work()
	}
	return a
}

func (a *AsyncMailer) work() {
	for m := range a.queue {
		if err := a.next.Send(context.Background(), m); err != nil {
			log.Printf("mail: sending to %s failed: %v", m.To, err)
		}
	}
}

// Send enqueues m; it fails only when the queue is full.
func (a *AsyncMailer) Send(_ context.Context, m Message) error {
	select {
	case a.queue <- m:
		return nil
	default:
		return errors.New("mail queue full, message dropped")
	}
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

// PasswordChanged builds the "your password was changed" notification.
func PasswordChanged(to, name string) Message {
	return Message{
		To: to, Subject: "Your password was changed",
		Text: fmt.Sprintf("Hi %s,\n\nThe password for your account was just changed and all your sessions were signed out.\n"+
			"If this was not you, reset your password immediately and contact support.\n", name),
		HTML: "<!doctype html><html><body><p>Hi " + htmltemplate.HTMLEscapeString(name) +
			",</p><p>The password for your account was just changed and all your sessions were signed out.</p>" +
			"<p>If this was not you, reset your password immediately and contact support.</p></body></html>",
	}
}
