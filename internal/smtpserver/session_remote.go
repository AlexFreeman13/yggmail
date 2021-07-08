package smtpserver

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-smtp"
)

type SessionRemote struct {
	backend    *Backend
	state      *smtp.ConnectionState
	public     ed25519.PublicKey
	from       string
	localparts []string
}

func (s *SessionRemote) Mail(from string, opts smtp.MailOptions) error {
	_, host, err := parseAddress(from)
	if err != nil {
		return fmt.Errorf("mail.ParseAddress: %w", err)
	}

	if local := s.state.RemoteAddr.String(); local != host {
		return fmt.Errorf("not allowed to send incoming mail as %s", from)
	}

	s.from = from
	return nil
}

func (s *SessionRemote) Rcpt(to string) error {
	user, host, err := parseAddress(to)
	if err != nil {
		return fmt.Errorf("mail.ParseAddress: %w", err)
	}

	if local := hex.EncodeToString(s.backend.Config.PublicKey); host != local {
		return fmt.Errorf("not allowed to send mail to %q", host)
	}

	s.localparts = append(s.localparts, user)
	return nil
}

func (s *SessionRemote) Data(r io.Reader) error {
	m, err := message.Read(r)
	if err != nil {
		return fmt.Errorf("message.Read: %w", err)
	}

	m.Header.Add(
		"Received", fmt.Sprintf("from Yggmail %s; %s",
			hex.EncodeToString(s.public),
			time.Now().String(),
		),
	)

	var b bytes.Buffer
	if err := m.WriteTo(&b); err != nil {
		return fmt.Errorf("m.WriteTo: %w", err)
	}

	for _, localpart := range s.localparts {
		if _, err := s.backend.Storage.MailCreate(localpart, "INBOX", b.Bytes()); err != nil {
			return fmt.Errorf("s.backend.Storage.StoreMessageFor: %w", err)
		}
		s.backend.Log.Printf("Stored new mail for local user %q from %s", localpart, s.from)
	}
	return nil
}

func (s *SessionRemote) Reset() {}

func (s *SessionRemote) Logout() error {
	return nil
}