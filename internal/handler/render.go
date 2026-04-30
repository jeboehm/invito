package handler

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jeboehm/invito/internal/db"
	"github.com/jeboehm/invito/internal/middleware"
	"github.com/jeboehm/invito/web"
)

// Each page template is parsed in its own isolated set so that
// {{define "content"}} blocks from different pages don't overwrite each other.
var templateDeps = map[string][]string{
	"landing.html":                   {"layout.html", "landing.html"},
	"booking/list.html":              {"layout.html", "booking/list.html"},
	"booking/picker.html":            {"layout.html", "booking/slots-partial.html", "booking/picker.html"},
	"booking/slots-partial.html":     {"booking/slots-partial.html"},
	"booking/confirm.html":           {"layout.html", "booking/confirm.html"},
	"dashboard/index.html":           {"layout.html", "dashboard/index.html"},
	"dashboard/calendars.html":       {"layout.html", "dashboard/index.html", "dashboard/calendars.html"},
	"dashboard/availability.html":    {"layout.html", "dashboard/index.html", "dashboard/availability.html"},
	"dashboard/event-types.html":     {"layout.html", "dashboard/index.html", "dashboard/event-types.html"},
	"dashboard/event-type-edit.html": {"layout.html", "dashboard/index.html", "dashboard/event-type-edit.html"},
	"dashboard/bookings.html":        {"layout.html", "dashboard/index.html", "dashboard/bookings.html"},
	"dashboard/profile.html":         {"layout.html", "dashboard/index.html", "dashboard/profile.html"},
}

var templates map[string]*template.Template

func init() {
	sub, err := fs.Sub(web.FS, "templates")
	if err != nil {
		panic(err)
	}
	funcs := template.FuncMap{
		"lower": strings.ToLower,
		"inTZ": func(t any, loc *time.Location) string {
			if loc == nil {
				loc = time.UTC
			}
			var tv time.Time
			switch v := t.(type) {
			case time.Time:
				tv = v
			case *time.Time:
				if v == nil {
					return ""
				}
				tv = *v
			default:
				return ""
			}
			return tv.In(loc).Format("Mon Jan 2, 2006 15:04")
		},
	}
	templates = make(map[string]*template.Template, len(templateDeps))
	for name, deps := range templateDeps {
		templates[name] = template.Must(template.New("").Funcs(funcs).ParseFS(sub, deps...))
	}
}

type baseData struct {
	User      *db.User
	CSRFToken string
	HideNav   bool
	NavActive string
}

func base(r *http.Request, user *db.User) baseData {
	return baseData{User: user, CSRFToken: middleware.CSRFToken(r)}
}

func baseDash(r *http.Request, user *db.User, nav string) baseData {
	b := base(r, user)
	b.NavActive = nav
	return b
}

func baseNoNav(r *http.Request) baseData {
	return baseData{CSRFToken: middleware.CSRFToken(r), HideNav: true}
}

var singleLineReplacer = strings.NewReplacer("\r", "", "\n", "")

// singleLine strips CR and LF so values are safe in single-line contexts (iCal, email headers).
func singleLine(s string) string { return singleLineReplacer.Replace(s) }

func render(w http.ResponseWriter, name string, data any) {
	t, ok := templates[name]
	if !ok {
		log.Printf("unknown template: %s", name)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template %s: %v", name, err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
