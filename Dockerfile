FROM golang:1.23-alpine AS build
WORKDIR /app

# Cache go modules
COPY go.mod ./
COPY go.su[m] ./
RUN go mod download || true

# Copy everything else
COPY . .

# Tidy in case go.sum is missing or stale
RUN go mod tidy

# Build: prefer ./cmd/server, fallback to ./cmd, fallback to root
RUN if [ -d ./cmd/server ]; then \
      CGO_ENABLED=0 go build -o /app/server ./cmd/server; \
    elif [ -d ./cmd ]; then \
      CGO_ENABLED=0 go build -o /app/server ./cmd; \
    else \
      CGO_ENABLED=0 go build -o /app/server .; \
    fi

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=build /app/server /server
EXPOSE 8080
CMD ["/server"]
