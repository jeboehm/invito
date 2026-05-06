package handler

import (
	"database/sql"
	"net/http"
	"net/url"
	"time"

	"github.com/jeboehm/invito/internal/auth"
	"github.com/jeboehm/invito/internal/booking"
	"github.com/jeboehm/invito/internal/calendar"
	"github.com/jeboehm/invito/internal/config"
	"github.com/jeboehm/invito/internal/db"
)

type PublicHandler struct {
	cfg     *config.Config
	db      *sql.DB
	booking *booking.Service
}

func NewPublicHandler(cfg *config.Config, database *sql.DB, bookingSvc *booking.Service) *PublicHandler {
	return &PublicHandler{cfg: cfg, db: database, booking: bookingSvc}
}

func (h *PublicHandler) HandleLanding(w http.ResponseWriter, r *http.Request) {
	render(w, "landing.html", base(r, auth.UserFromContext(r.Context())))
}

type bookingListData struct {
	baseData
	EventTypes []db.EventType
}

func (h *PublicHandler) HandleUserBookingPage(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	user, err := db.GetUserByUsername(h.db, username)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	ets, _ := db.ListEventTypes(h.db, user.ID)
	var active []db.EventType
	for _, et := range ets {
		if et.Active {
			active = append(active, et)
		}
	}

	bd := base(r, user)
	bd.HideNav = true
	render(w, "booking/list.html", bookingListData{bd, active})
}

type pickerData struct {
	baseData
	EventType    *db.EventType
	Dates        []time.Time
	SelectedDate string
	Slots        []calendar.Slot
	HostTimezone string // IANA name shown to guests, e.g. "Europe/Berlin"
}

func (h *PublicHandler) HandleSlotPicker(w http.ResponseWriter, r *http.Request) {
	h.handleSlotPicker(w, r, "booking/picker.html", "booking/slots-partial.html")
}

func (h *PublicHandler) handleSlotPicker(w http.ResponseWriter, r *http.Request, fullTmpl, partialTmpl string) {
	username := r.PathValue("username")
	slug := r.PathValue("slug")

	user, err := db.GetUserByUsername(h.db, username)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	et, err := db.GetEventTypeBySlug(h.db, user.ID, slug)
	if err == sql.ErrNoRows || (err == nil && !et.Active) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	hostLoc := user.Location()
	now := time.Now().In(hostLoc)
	selectedDate := r.URL.Query().Get("date")
	var date time.Time
	if selectedDate != "" {
		date, err = time.ParseInLocation("2006-01-02", selectedDate, hostLoc)
		if err != nil {
			date = now
		}
	} else {
		date = now
		selectedDate = date.Format("2006-01-02")
	}
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, hostLoc)

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, hostLoc)
	dates := make([]time.Time, 7)
	for i := range dates {
		dates[i] = today.Add(time.Duration(i) * 24 * time.Hour)
	}

	bd := base(r, user)
	bd.HideNav = true
	data := pickerData{
		baseData:     bd,
		EventType:    et,
		Dates:        dates,
		SelectedDate: selectedDate,
		Slots:        h.calculateSlots(user, et, date),
		HostTimezone: hostLoc.String(),
	}

	if isHTMX(r) {
		render(w, partialTmpl, data)
		return
	}
	render(w, fullTmpl, data)
}

type confirmData struct {
	baseData
	Icon    string
	Title   string
	Message string
}

func (h *PublicHandler) HandleBookingSubmit(w http.ResponseWriter, r *http.Request) {
	h.handleBookingSubmit(w, r, "booking/confirm.html")
}

func (h *PublicHandler) handleBookingSubmit(w http.ResponseWriter, r *http.Request, confirmTmpl string) {
	username := r.PathValue("username")
	slug := r.PathValue("slug")

	user, err := db.GetUserByUsername(h.db, username)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	et, err := db.GetEventTypeBySlug(h.db, user.ID, slug)
	if err != nil || !et.Active {
		http.NotFound(w, r)
		return
	}

	r.ParseForm()
	slotStr := r.FormValue("slot")
	startAt, err := time.Parse(time.RFC3339, slotStr)
	if err != nil {
		http.Error(w, "invalid slot", http.StatusBadRequest)
		return
	}
	endAt := startAt.Add(time.Duration(et.DurationMinutes) * time.Minute)

	hostLoc := user.Location()
	date := time.Date(startAt.Year(), startAt.Month(), startAt.Day(), 0, 0, 0, 0, hostLoc)
	available := h.calculateSlots(user, et, date)
	validSlot := false
	for _, s := range available {
		if s.Start.Equal(startAt) {
			validSlot = true
			break
		}
	}
	if !validSlot {
		http.Error(w, "slot is no longer available", http.StatusConflict)
		return
	}

	b := &db.Booking{
		EventTypeID:   et.ID,
		GuestName:     singleLine(r.FormValue("guest_name")),
		GuestEmail:    singleLine(r.FormValue("guest_email")),
		GuestNote:     r.FormValue("guest_note"),
		StartAt:       startAt,
		EndAt:         endAt,
		Token:         randomHex(16),
		ReservedUntil: time.Now().Add(h.cfg.BookingTTL),
	}

	if err := h.booking.CreateBooking(r.Context(), b, et, user); err != nil {
		http.Error(w, "could not create booking: "+err.Error(), http.StatusConflict)
		return
	}

	render(w, confirmTmpl, confirmData{
		baseNoNav(r),
		"📅",
		"Booking request sent!",
		"Your request has been submitted. " + user.Name + " will confirm or reject it within 24 hours. You'll receive an email either way.",
	})
}

func (h *PublicHandler) HandleBookingConfirm(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	b, err := h.booking.ConfirmBooking(r.Context(), token)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not confirm booking", http.StatusInternalServerError)
		return
	}

	msg := "Booking confirmed."
	if b.Status != "CONFIRMED" {
		msg = "This booking has already been processed."
	}

	http.Redirect(w, r, "/dashboard/bookings?flash="+url.QueryEscape(msg), http.StatusSeeOther)
}

func (h *PublicHandler) HandleBookingReject(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	_, err := h.booking.RejectBooking(r.Context(), token)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not reject booking", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/dashboard/bookings?flash="+url.QueryEscape("Booking rejected."), http.StatusSeeOther)
}

func (h *PublicHandler) calculateSlots(user *db.User, et *db.EventType, date time.Time) []calendar.Slot {
	now := time.Now()
	from := now.Add(-time.Hour)
	to := now.Add(time.Duration(et.BookingWindowDays) * 24 * time.Hour)

	rules, _ := db.ListAvailabilityRules(h.db, user.ID)
	events, _ := db.ListCalendarEventsForUser(h.db, user.ID, from, to)
	bookings, _ := db.ListPendingBookingsInRange(h.db, user.ID, from, to)

	return calendar.CalculateSlots(
		rules, events, bookings,
		date,
		time.Duration(et.DurationMinutes)*time.Minute,
		user.Location(),
		et.BookingWindowDays,
		now,
	)
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
