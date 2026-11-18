package mail

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeServer is a minimal SMTP server without TLS and without login. It
// accepts one message and passes its text on.
func fakeServer(t *testing.T) (host, port string, received <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	out := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		reply := func(line string) { conn.Write([]byte(line + "\r\n")) }

		reply("220 test server")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			switch cmd := strings.ToUpper(strings.TrimSpace(line)); {
			case strings.HasPrefix(cmd, "DATA"):
				reply("354 go ahead")
				var msg strings.Builder
				for {
					l, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if l == ".\r\n" {
						break
					}
					msg.WriteString(l)
				}
				out <- msg.String()
				reply("250 accepted")
			case strings.HasPrefix(cmd, "QUIT"):
				reply("221 bye")
				return
			default: // EHLO, MAIL, RCPT
				reply("250 ok")
			}
		}
	}()

	host, port, err = net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split address: %v", err)
	}
	return host, port, out
}

func TestSendDeliversMessageWithEncodedSubject(t *testing.T) {
	host, port, received := fakeServer(t)
	s := SMTPSender{Host: host, Port: port, From: "oasis@example.com"}

	if err := s.Send(context.Background(), "bob@example.com", "Passwort zurücksetzen", "Dein Code: abc\n"); err != nil {
		t.Fatalf("send: %v", err)
	}

	msg := <-received
	for _, want := range []string{
		"From: oasis@example.com\r\n",
		"To: bob@example.com\r\n",
		"Subject: =?utf-8?q?Passwort_zur=C3=BCcksetzen?=\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"\r\n\r\nDein Code: abc\r\n",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message does not contain %q:\n%s", want, msg)
		}
	}
}

func TestSendRejectsLineBreaksInHeader(t *testing.T) {
	s := SMTPSender{Host: "127.0.0.1", Port: "1", From: "oasis@example.com"}
	err := s.Send(context.Background(), "bob@example.com\r\nBcc: eve@example.com", "Hi", "text")
	if err == nil {
		t.Fatal("expected an error for a recipient with a line break")
	}
}

func TestSendGivesUpWhenTheServerDoesNotAnswer(t *testing.T) {
	// This server accepts the connection and then stays silent.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	host, port, _ := net.SplitHostPort(ln.Addr().String())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- SMTPSender{Host: host, Port: port, From: "oasis@example.com"}.Send(ctx, "bob@example.com", "Hi", "text")
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("got %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Send still blocks long after its context has ended")
	}
}
