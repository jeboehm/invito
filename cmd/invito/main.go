package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/jeboehm/invito/internal/auth"
	"github.com/jeboehm/invito/internal/booking"
	"github.com/jeboehm/invito/internal/calendar"
	"github.com/jeboehm/invito/internal/config"
	"github.com/jeboehm/invito/internal/db"
	"github.com/jeboehm/invito/internal/email"
	"github.com/jeboehm/invito/internal/handler"
	"github.com/jeboehm/invito/internal/middleware"
	"github.com/jeboehm/invito/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Init OIDC provider (fails fast if unreachable)
	oidcProvider, err := auth.NewProvider(ctx, cfg)
	if err != nil {
		log.Fatalf("oidc: %v", err)
	}

	mailer := email.NewMailer(cfg)
	bookingSvc := booking.NewService(database, mailer, cfg.BaseURL, cfg.SessionSecret)

	// Handlers
	authH := handler.NewAuthHandler(cfg, database, oidcProvider)
	dashH := handler.NewDashboardHandler(cfg, database)
	pubH := handler.NewPublicHandler(cfg, database, bookingSvc)

	requireAuth := auth.RequireAuth(database)
	optionalAuth := auth.OptionalAuth(database)

	mux := http.NewServeMux()

	// Static files
	staticFS, _ := fs.Sub(web.FS, "static")
	mux.Handle("GET /static/{path...}", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Public routes
	mux.Handle("GET /", optionalAuth(http.HandlerFunc(pubH.HandleLanding)))
	mux.HandleFunc("GET /calendar/{username}/", pubH.HandleUserBookingPage)
	mux.HandleFunc("GET /calendar/{username}/{slug}", pubH.HandleSlotPicker)
	bookingLimiter := middleware.RateLimit(ctx, 10.0/60, 5)
	mux.Handle("POST /calendar/{username}/{slug}/book", bookingLimiter(http.HandlerFunc(pubH.HandleBookingSubmit)))
	mux.HandleFunc("GET /booking/{token}/confirm", pubH.HandleBookingConfirm)
	mux.HandleFunc("GET /booking/{token}/reject", pubH.HandleBookingReject)

	// Auth routes
	mux.HandleFunc("GET /auth/login", authH.HandleLogin)
	mux.HandleFunc("GET /auth/callback", authH.HandleCallback)
	mux.HandleFunc("POST /auth/logout", authH.HandleLogout)

	// Dashboard routes (auth required)
	dash := func(method, pattern string, h http.HandlerFunc) {
		mux.Handle(method+" "+pattern, requireAuth(h))
	}
	dash("GET", "/dashboard", dashH.HandleIndex)
	dash("GET", "/dashboard/calendars", dashH.HandleCalendarsGet)
	dash("POST", "/dashboard/calendars", dashH.HandleCalendarsPost)
	dash("DELETE", "/dashboard/calendars/{id}", dashH.HandleCalendarDelete)
	dash("POST", "/dashboard/calendars/{id}/sync", dashH.HandleCalendarSync)
	dash("GET", "/dashboard/availability", dashH.HandleAvailabilityGet)
	dash("POST", "/dashboard/availability", dashH.HandleAvailabilityPost)
	dash("GET", "/dashboard/event-types", dashH.HandleEventTypesGet)
	dash("POST", "/dashboard/event-types", dashH.HandleEventTypesPost)
	dash("GET", "/dashboard/event-types/{id}/edit", dashH.HandleEventTypeEditGet)
	dash("POST", "/dashboard/event-types/{id}", dashH.HandleEventTypePost)
	dash("POST", "/dashboard/event-types/{id}/toggle", dashH.HandleEventTypeToggle)
	dash("GET", "/dashboard/bookings", dashH.HandleBookingsGet)
	dash("GET", "/dashboard/profile", dashH.HandleProfileGet)
	dash("POST", "/dashboard/profile", dashH.HandleProfilePost)

	// Global middleware
	rootHandler := middleware.Logging(middleware.SecurityHeaders(middleware.CSRF(strings.HasPrefix(cfg.BaseURL, "https://"))(mux)))

	// Background jobs
	go calendar.StartSyncLoop(ctx, database, cfg.SessionSecret, cfg.SyncInterval)
	go bookingSvc.StartGCLoop(ctx)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           rootHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx)
	}()

	log.Printf("invito listening on %s", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
