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

.PHONY: run build install dashboard build-go install-go
