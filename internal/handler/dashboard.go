package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jeboehm/invito/internal/auth"
	"github.com/jeboehm/invito/internal/calendar"
	"github.com/jeboehm/invito/internal/config"
	"github.com/jeboehm/invito/internal/crypto"
	"github.com/jeboehm/invito/internal/db"
)

const syncWindowDays = 60

type DashboardHandler struct {
	cfg *config.Config
	db  *sql.DB
}

func NewDashboardHandler(cfg *config.Config, database *sql.DB) *DashboardHandler {
	return &DashboardHandler{cfg: cfg, db: database}
}

// --- Overview ---

type dashboardData struct {
	baseData
	PendingCount int
	Upcoming     []db.BookingWithEventType
}

func (h *DashboardHandler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	count, _ := db.CountPendingForUser(h.db, user.ID)
	upcoming, _ := db.ListUpcomingConfirmedForUser(h.db, user.ID, 5)
	render(w, "dashboard/index.html", dashboardData{baseDash(r, user, "overview"), count, upcoming})
}

//go:generate go run gen_timezones.go

// --- Profile ---

type profileData struct {
	baseData
	BaseURL   string
	Timezones []string
	Error     string
}

func (h *DashboardHandler) HandleProfileGet(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	render(w, "dashboard/profile.html", profileData{baseDash(r, user, "profile"), h.cfg.BaseURL, allTimezones, ""})
}

func (h *DashboardHandler) HandleProfilePost(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(singleLine(r.FormValue("name")))
	username := slugify(r.FormValue("username"))
	timezone := r.FormValue("timezone")
	if timezone == "" {
		timezone = "UTC"
	}

	showError := func(msg string) {
		render(w, "dashboard/profile.html", profileData{baseDash(r, user, "profile"), h.cfg.BaseURL, allTimezones, msg})
	}

	if name == "" {
		showError("Display name cannot be empty")
		return
	}
	if username == "" {
		showError("Username slug cannot be empty")
		return
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		showError("Invalid timezone")
		return
	}

	if username != user.Username {
		existing, err := db.GetUserByUsername(h.db, username)
		if err == nil && existing.ID != user.ID {
			showError("This username is already taken")
			return
		}
	}

	if err := db.UpdateUserProfile(h.db, user.ID, name, username, timezone); err != nil {
		showError("Could not save profile")
		return
	}

	http.Redirect(w, r, "/dashboard/profile", http.StatusSeeOther)
}

// --- Calendars ---

type calendarsData struct {
	baseData
	Calendars []db.CalendarSummary
	Error     string
}

func (h *DashboardHandler) HandleCalendarsGet(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	cals, _ := db.ListCalendarsWithEventCounts(h.db, user.ID)
	render(w, "dashboard/calendars.html", calendarsData{baseDash(r, user, "calendars"), cals, ""})
}

func (h *DashboardHandler) HandleCalendarsPost(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	r.ParseForm()

	calURL := strings.TrimRight(singleLine(r.FormValue("caldav_url")), "/")
	calUser := singleLine(r.FormValue("username"))
	calPass := singleLine(r.FormValue("password"))
	displayName := singleLine(r.FormValue("display_name"))
	color := singleLine(r.FormValue("color"))
	if color == "" {
		color = "#6366f1"
	}

	showError := func(msg string) {
		cals, _ := db.ListCalendarsWithEventCounts(h.db, user.ID)
		render(w, "dashboard/calendars.html", calendarsData{baseDash(r, user, "calendars"), cals, msg})
	}

	if err := calendar.VerifyCredentials(r.Context(), calURL, calUser, calPass); err != nil {
		showError(fmt.Sprintf("Could not connect to CalDAV server: %v", err))
		return
	}

	encPass, err := crypto.Encrypt(h.cfg.SessionSecret, []byte(calPass))
	if err != nil {
		showError("Encryption error")
		return
	}

	_, err = db.CreateCalendar(h.db, &db.Calendar{
		UserID:      user.ID,
		CalDAVURL:   calURL,
		Username:    calUser,
		Password:    string(encPass),
		DisplayName: displayName,
		Color:       color,
	})
	if err != nil {
		showError("Could not save calendar")
		return
	}

	http.Redirect(w, r, "/dashboard/calendars", http.StatusSeeOther)
}

func (h *DashboardHandler) HandleCalendarDelete(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	db.DeleteCalendar(h.db, id, user.ID)
	// HTMX: return empty 200 (the row removes itself via hx-swap="outerHTML")
	w.WriteHeader(http.StatusOK)
}

func (h *DashboardHandler) HandleCalendarSync(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	cal, err := db.GetCalendar(h.db, id, user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	now := time.Now()
	from := now.Add(-time.Hour)
	to := now.Add(syncWindowDays * 24 * time.Hour)

	if err := calendar.SyncCalendar(r.Context(), cal, h.cfg.SessionSecret, h.db, from, to); err != nil {
		http.Error(w, fmt.Sprintf("sync failed: %v", err), http.StatusInternalServerError)
		return
	}

	db.UpdateLastSynced(h.db, id, now)
	http.Redirect(w, r, "/dashboard/calendars", http.StatusSeeOther)
}

// --- Availability ---

type availabilityData struct {
	baseData
	Rules    []db.AvailabilityRule
	Weekdays []string
}

var weekdayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

func (h *DashboardHandler) HandleAvailabilityGet(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	rules, _ := db.ListAvailabilityRules(h.db, user.ID)
	// Ensure we have a row for each weekday (fill gaps)
	byDay := make(map[int]db.AvailabilityRule)
	for _, rule := range rules {
		byDay[rule.Weekday] = rule
	}
	full := make([]db.AvailabilityRule, 7)
	for i := 0; i < 7; i++ {
		if r, ok := byDay[i]; ok {
			full[i] = r
		} else {
			full[i] = db.AvailabilityRule{Weekday: i, StartTime: "09:00", EndTime: "17:00"}
		}
	}
	render(w, "dashboard/availability.html", availabilityData{baseDash(r, user, "availability"), full, weekdayNames})
}

func (h *DashboardHandler) HandleAvailabilityPost(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	r.ParseForm()

	var rules []db.AvailabilityRule
	for i := 0; i < 7; i++ {
		key := fmt.Sprintf("day_%d", i)
		active := r.FormValue(key+"_active") == "on"
		start := r.FormValue(key + "_start")
		end := r.FormValue(key + "_end")
		if start == "" || end == "" {
			continue
		}
		rules = append(rules, db.AvailabilityRule{
			UserID:    user.ID,
			Weekday:   i,
			StartTime: start,
			EndTime:   end,
			Active:    active,
		})
	}

	if err := db.ReplaceAvailabilityRules(h.db, user.ID, rules); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dashboard/availability", http.StatusSeeOther)
}

// --- Event Types ---

type eventTypesData struct {
	baseData
	EventTypes []db.EventType
	BaseURL    string
	Error      string
}

func (h *DashboardHandler) HandleEventTypesGet(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	ets, _ := db.ListEventTypes(h.db, user.ID)
	render(w, "dashboard/event-types.html", eventTypesData{baseDash(r, user, "event-types"), ets, h.cfg.BaseURL, ""})
}

func (h *DashboardHandler) HandleEventTypesPost(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	r.ParseForm()

	slug := slugify(r.FormValue("slug"))
	title := singleLine(r.FormValue("title"))
	description := r.FormValue("description")
	duration, _ := strconv.Atoi(r.FormValue("duration_minutes"))
	color := singleLine(r.FormValue("color"))
	if color == "" {
		color = "#6366f1"
	}
	bookingWindow, err := strconv.Atoi(r.FormValue("booking_window_days"))
	if err != nil || bookingWindow <= 0 {
		bookingWindow = 60
	}

	showError := func(msg string) {
		ets, _ := db.ListEventTypes(h.db, user.ID)
		render(w, "dashboard/event-types.html", eventTypesData{baseDash(r, user, "event-types"), ets, h.cfg.BaseURL, msg})
	}

	if slug == "" || title == "" || duration <= 0 {
		showError("Title, slug, and duration are required")
		return
	}

	_, err = db.CreateEventType(h.db, &db.EventType{
		UserID:            user.ID,
		Slug:              slug,
		Title:             title,
		Description:       description,
		DurationMinutes:   duration,
		Color:             color,
		BookingWindowDays: bookingWindow,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			showError("An event type with this slug already exists")
		} else {
			showError("Could not create event type")
		}
		return
	}

	http.Redirect(w, r, "/dashboard/event-types", http.StatusSeeOther)
}

func (h *DashboardHandler) HandleEventTypeEditGet(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	et, err := db.GetEventType(h.db, id, user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	render(w, "dashboard/event-type-edit.html", struct {
		baseData
		EventType *db.EventType
		Error     string
	}{baseDash(r, user, "event-types"), et, ""})
}

func (h *DashboardHandler) HandleEventTypePost(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	et, err := db.GetEventType(h.db, id, user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	r.ParseForm()
	et.Title = singleLine(r.FormValue("title"))
	et.Description = r.FormValue("description")
	et.ConfirmedMessage = r.FormValue("confirmed_message")
	et.RejectedMessage = r.FormValue("rejected_message")
	et.DurationMinutes, _ = strconv.Atoi(r.FormValue("duration_minutes"))
	et.Color = singleLine(r.FormValue("color"))
	bw, err := strconv.Atoi(r.FormValue("booking_window_days"))
	if err == nil && bw > 0 {
		et.BookingWindowDays = bw
	}

	if err := db.UpdateEventType(h.db, et); err != nil {
		render(w, "dashboard/event-type-edit.html", struct {
			baseData
			EventType *db.EventType
			Error     string
		}{baseDash(r, user, "event-types"), et, "Could not update event type"})
		return
	}

	http.Redirect(w, r, "/dashboard/event-types", http.StatusSeeOther)
}

func (h *DashboardHandler) HandleEventTypeToggle(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	db.ToggleEventType(h.db, id, user.ID)
	http.Redirect(w, r, "/dashboard/event-types", http.StatusSeeOther)
}

// --- Bookings ---

type bookingsData struct {
	baseData
	Bookings      []db.BookingWithEventType
	StatusFilter  string
	StatusOptions []string
}

func (h *DashboardHandler) HandleBookingsGet(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	status := r.URL.Query().Get("status")
	bookings, _ := db.ListBookingsForUser(h.db, user.ID, status, 100)
	render(w, "dashboard/bookings.html", bookingsData{
		baseDash(r, user, "bookings"),
		bookings,
		status,
		[]string{"", "PENDING", "CONFIRMED", "REJECTED", "CANCELLED"},
	})
}

func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
