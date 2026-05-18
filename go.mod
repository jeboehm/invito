module github.com/jeboehm/invito

go 1.26

require (
	github.com/coreos/go-oidc/v3 v3.18.0
	github.com/emersion/go-ical v0.0.0-20250609112844-439c63cef608
	github.com/emersion/go-webdav v0.7.0 // replaced below; remove once upstream PR #205 is merged
	github.com/playwright-community/playwright-go v0.5700.1
	golang.org/x/oauth2 v0.36.0
	golang.org/x/time v0.15.0
	modernc.org/sqlite v1.50.1
)

require (
	github.com/deckarep/golang-set/v2 v2.8.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-jose/go-jose/v3 v3.0.5 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/teambition/rrule-go v1.8.2 // indirect
	golang.org/x/sys v0.42.0 // indirect
	modernc.org/libc v1.72.0 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)

replace github.com/emersion/go-webdav => ./go-webdav
