PORT ?= 8090

run:
	go run . http $(PORT)

# Build the embedded Next.js dashboard, then build the goport binary.
build: dashboard
	go build -o goport.exe .

# Same as build, but install to GOPATH/bin so the `goport` command updates.
install: dashboard
	go build -o $(shell go env GOPATH)/bin/goport.exe .

# Build only the dashboard static export and copy it into the embed dir.
dashboard:
	cd dashboard && npx --no-install next build
	rm -rf tunnel/dashboard_dist
	cp -r dashboard/out tunnel/dashboard_dist

# Build the Go binary without rebuilding the dashboard (faster iteration).
build-go:
	go build -o goport.exe .

install-go:
	go build -o $(shell go env GOPATH)/bin/goport.exe .

# ---- Release builds (cross-compiled binaries for GitHub releases) -------------
# Output goes to dist/ with names matching the GitHub release assets. The
# dashboard is rebuilt once and embedded into every binary.

# Build the Windows .exe release binary into dist/.
release-windows: dashboard
	GOOS=windows GOARCH=amd64 go build -o dist/goport-windows-amd64.exe .

# Build the Linux release binary into dist/.
release-linux: dashboard
	GOOS=linux GOARCH=amd64 go build -o dist/goport-linux-amd64 .

# Build the macOS release binaries (Intel + Apple Silicon) into dist/.
release-darwin: dashboard
	GOOS=darwin GOARCH=amd64 go build -o dist/goport-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o dist/goport-darwin-arm64 .

# Build every release binary (all platforms) into dist/. Rebuilds the dashboard
# once, then cross-compiles for each target.
release: dashboard
	GOOS=windows GOARCH=amd64 go build -o dist/goport-windows-amd64.exe .
	GOOS=linux   GOARCH=amd64 go build -o dist/goport-linux-amd64 .
	GOOS=darwin  GOARCH=amd64 go build -o dist/goport-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -o dist/goport-darwin-arm64 .

.PHONY: run build install dashboard build-go install-go release release-windows release-linux release-darwin
