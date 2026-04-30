//go:build e2e

// Package e2e contains end-to-end tests for Invito.
//
// Prerequisites (must be running before `go test -tags e2e ./e2e/`):
//
//	docker compose up -d dex mailpit
//
// The tests start an Invito server on :8080 (matching Dex's allowed redirect URI).
// Do not run with `docker compose up invito` — port 8080 must be free.
package e2e_test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	playwright "github.com/playwright-community/playwright-go"
)

// serverURL is the base URL of the test Invito instance.
// Must match the redirect URI registered in dev/dex/config.yaml.
const serverURL = "http://localhost:8080"

// pw and browser are shared across all tests, initialized in TestMain.
var (
	pw      *playwright.Playwright
	browser playwright.Browser
)

func TestMain(m *testing.M) {
	os.Exit(testMain(m))
}

func testMain(m *testing.M) int {
	dbFile, err := os.CreateTemp("", "invito-e2e-*.db")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create temp db:", err)
		return 1
	}
	dbFile.Close()
	defer os.Remove(dbFile.Name())

	// Build the binary once so subsequent test runs don't pay compile time.
	// CWD during `go test ./e2e/` is the e2e/ package directory, so .. is the module root.
	binary, err := os.CreateTemp("", "invito-e2e-bin-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create temp binary:", err)
		return 1
	}
	binary.Close()
	binaryPath := binary.Name()
	defer os.Remove(binaryPath)

	fmt.Println("e2e: compiling server binary…")
	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/invito")
	build.Dir = ".."
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "build server:", err)
		return 1
	}

	// Setpgid puts the child in its own process group so we can kill the entire
	// group on cleanup — the binary may spawn goroutines but no child processes.
	cmd := exec.Command(binaryPath)
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(),
		"INVITO_BASE_URL="+serverURL,
		"INVITO_DB_PATH="+dbFile.Name(),
		"INVITO_LISTEN_ADDR=:8080",
		"INVITO_SESSION_SECRET=0000000000000000000000000000000000000000000000000000000000000001",
		"INVITO_OIDC_ISSUER=http://localhost:5556",
		"INVITO_OIDC_CLIENT_ID=invito",
		"INVITO_OIDC_CLIENT_SECRET=invito-dev-secret",
		"INVITO_SMTP_HOST=localhost",
		"INVITO_SMTP_PORT=1025",
		"INVITO_SMTP_FROM=invito@localhost",
		"INVITO_SYNC_INTERVAL=1h",
		"INVITO_BOOKING_TTL=24h",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "start server:", err)
		return 1
	}
	defer func() {
		// Kill the entire process group to ensure the invito child process exits too.
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) //nolint:errcheck
			cmd.Wait()                                      //nolint:errcheck
		}
	}()

	fmt.Println("e2e: waiting for server to be ready…")
	if !waitReady(serverURL+"/", 15*time.Second) {
		fmt.Fprintln(os.Stderr, "e2e: server did not become ready within 15s")
		return 1
	}
	fmt.Println("e2e: server ready")

	// Install playwright browsers if not already present. Idempotent — fast when cached.
	fmt.Println("e2e: ensuring playwright browsers are installed…")
	if err := playwright.Install(&playwright.RunOptions{
		Browsers: []string{"chromium"},
		Verbose:  false,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "playwright install:", err)
		return 1
	}

	pw, err = playwright.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "playwright run:", err)
		return 1
	}
	defer pw.Stop() //nolint:errcheck

	browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(os.Getenv("CHROME_HEADLESS") != "0"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch browser:", err)
		return 1
	}
	defer browser.Close()

	return m.Run()
}

func waitReady(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec
		if err == nil {
			resp.Body.Close()
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}
