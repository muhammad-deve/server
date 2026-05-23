FROM golang:1.23-bullseye AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -ldflags="-linkmode external -extldflags -static" -tags netgo -o /app/main ./cmd/server/main.go

FROM alpine:3.19

COPY --from=build /app/main /main

EXPOSE 8080

CMD ["/main"]
