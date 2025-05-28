FROM golang:1.23 AS builder

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o app ./...

FROM gcr.io/distroless/base-debian12

WORKDIR /app
COPY --from=builder /usr/src/app/app /app/app

USER nonroot:nonroot

CMD ["/app/app"]