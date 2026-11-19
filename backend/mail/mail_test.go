package mail_test

import (
	"bufio"
	"context"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/lingster/go-rest-template/backend/mail"
)

// plaintextSMTP accepts one connection and speaks just enough SMTP to advertise no STARTTLS.
func plaintextSMTP(t *testing.T) (host string, port int, got chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	got = make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		w := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
		w("220 test ESMTP")
		var data strings.Builder
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				got <- data.String()
				return
			}
			if inData {
				if line == ".\r\n" {
					inData = false
					w("250 ok")
					continue
				}
				data.WriteString(line)
				continue
			}
			switch cmd := strings.ToUpper(strings.TrimSpace(line)); {
			case strings.HasPrefix(cmd, "EHLO"):
				w("250-test")
				w("250 8BITMIME")
			case strings.HasPrefix(cmd, "DATA"):
				inData = true
				w("354 go ahead")
			case strings.HasPrefix(cmd, "QUIT"):
				w("221 bye")
				got <- data.String()
				return
			default:
				w("250 ok")
			}
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port, got
}

func msg() mail.Message {
	return mail.ResetPassword("user@example.com", "User", "https://app.example.com/reset-password?token=secret")
}

func TestSMTPRequireTLSRefusesPlaintext(t *testing.T) {
	host, port, got := plaintextSMTP(t)
	m := &mail.SMTPMailer{Host: host, Port: port, From: "App <no-reply@example.com>", RequireTLS: true}
	if err := m.Send(context.Background(), msg()); err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("expected STARTTLS error, got %v", err)
	}
	if body := <-got; strings.Contains(body, "token=secret") {
		t.Fatal("reset link was sent in plaintext")
	}
}

func TestSMTPPlaintextAllowedWhenNotRequired(t *testing.T) {
	host, port, got := plaintextSMTP(t)
	m := &mail.SMTPMailer{Host: host, Port: port, From: "App <no-reply@example.com>"}
	if err := m.Send(context.Background(), msg()); err != nil {
		t.Fatalf("send: %v", err)
	}
	if body := <-got; !strings.Contains(body, "token=secret") {
		t.Fatalf("message not delivered: %q", body)
	}
}

type blockingMailer struct{ release chan struct{} }

func (b blockingMailer) Send(context.Context, mail.Message) error { <-b.release; return nil }

func TestAsyncMailerNeverBlocksCaller(t *testing.T) {
	b := blockingMailer{release: make(chan struct{})}
	defer close(b.release)
	a := mail.NewAsync(b, 1, 2)
	var errs int
	for i := 0; i < 10; i++ { // 1 in flight + 2 queued; the rest are dropped, not blocked
		if err := a.Send(context.Background(), mail.Message{To: "u" + strconv.Itoa(i) + "@example.com"}); err != nil {
			errs++
		}
	}
	if errs == 0 {
		t.Fatal("a full queue must drop instead of blocking")
	}
}
