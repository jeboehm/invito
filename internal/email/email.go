package email

import (
	"bytes"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/smtp"
	"net/textproto"

	"github.com/jboehm/invito/internal/config"
)

type Mailer struct {
	host     string
	port     int
	user     string
	password string
	from     string
}

func NewMailer(cfg *config.Config) *Mailer {
	return &Mailer{
		host:     cfg.SMTPHost,
		port:     cfg.SMTPPort,
		user:     cfg.SMTPUser,
		password: cfg.SMTPPassword,
		from:     cfg.SMTPFrom,
	}
}

func (m *Mailer) Send(to, subject, textBody, htmlBody string) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	headers := textproto.MIMEHeader{}
	headers.Set("From", m.from)
	headers.Set("To", to)
	headers.Set("Subject", mime.QEncoding.Encode("utf-8", subject))
	headers.Set("MIME-Version", "1.0")
	headers.Set("Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", w.Boundary()))

	for k, vs := range headers {
		for _, v := range vs {
			buf.WriteString(k + ": " + v + "\r\n")
		}
	}
	buf.WriteString("\r\n")

	// text/plain part
	ph := textproto.MIMEHeader{}
	ph.Set("Content-Type", "text/plain; charset=utf-8")
	ph.Set("Content-Transfer-Encoding", "quoted-printable")
	plainPart, err := w.CreatePart(ph)
	if err != nil {
		return err
	}
	qw := quotedprintable.NewWriter(plainPart)
	qw.Write([]byte(textBody))
	qw.Close()

	// text/html part
	ph = textproto.MIMEHeader{}
	ph.Set("Content-Type", "text/html; charset=utf-8")
	ph.Set("Content-Transfer-Encoding", "quoted-printable")
	htmlPart, err := w.CreatePart(ph)
	if err != nil {
		return err
	}
	qw = quotedprintable.NewWriter(htmlPart)
	qw.Write([]byte(htmlBody))
	qw.Close()

	w.Close()

	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	var auth smtp.Auth
	if m.user != "" {
		auth = smtp.PlainAuth("", m.user, m.password, m.host)
	}
	return smtp.SendMail(addr, auth, m.from, []string{to}, buf.Bytes())
}
