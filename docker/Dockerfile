FROM golang:1.27.1-alpine AS builder
ARG TARGETARCH

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
ENV GOOS=linux GOARCH=${TARGETARCH} CGO_ENABLED=0
RUN go build -o job-applications main.go
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.27.3

# Final image
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/job-applications .
COPY --from=builder /go/bin/goose ./goose
COPY --from=builder /app/internal/db/schema ./schema

COPY static ./static
COPY templates ./templates

RUN apk update && apk add curl tzdata

HEALTHCHECK --interval=30s --timeout=30s --retries=3 CMD curl -f http://localhost/health

EXPOSE 9000

CMD ["./job-applications"]