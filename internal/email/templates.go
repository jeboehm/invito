package email

import (
	"bytes"
	"embed"
	"html/template"
	"time"
	texttemplate "text/template"
)

//go:embed mail_templates
var mailFS embed.FS

var (
	htmlTmpl *template.Template
	textTmpl *texttemplate.Template
)

func init() {
	htmlTmpl = template.Must(template.New("").ParseFS(mailFS, "mail_templates/*.html"))
	textTmpl = texttemplate.Must(texttemplate.New("").ParseFS(mailFS, "mail_templates/*.txt"))
}

type BookingEmailData struct {
	GuestName    string
	GuestEmail   string
	GuestNote    string
	HostName     string
	EventTitle   string
	GuestMessage string
	StartAt      time.Time
	ConfirmURL   string
	RejectURL    string
}

func RenderBookingCreated(data BookingEmailData) (text, html string, err error) {
	return render("booking-created.txt", "booking-created.html", data)
}

func RenderBookingConfirmed(data BookingEmailData) (text, html string, err error) {
	return render("booking-confirmed.txt", "booking-confirmed.html", data)
}

func RenderBookingRejected(data BookingEmailData) (text, html string, err error) {
	return render("booking-rejected.txt", "booking-rejected.html", data)
}

func RenderBookingCancelled(data BookingEmailData) (text, html string, err error) {
	return render("booking-cancelled.txt", "booking-cancelled.html", data)
}

func render(textName, htmlName string, data any) (string, string, error) {
	var textBuf, htmlBuf bytes.Buffer
	if err := textTmpl.ExecuteTemplate(&textBuf, textName, data); err != nil {
		return "", "", err
	}
	if err := htmlTmpl.ExecuteTemplate(&htmlBuf, htmlName, data); err != nil {
		return "", "", err
	}
	return textBuf.String(), htmlBuf.String(), nil
}
