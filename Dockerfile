FROM golang:1.26.5-alpine AS builder
LABEL stage=gobuilder

ENV CGO_ENABLED=0
ENV GOOS=linux

RUN apk update && apk upgrade && apk add --no-cache ca-certificates
RUN update-ca-certificates

COPY . /app

WORKDIR /app

RUN go mod download
RUN go build -ldflags="-w -s" -o application ./main.go

# ------------------------------------------------------------------ #

FROM alpine:3.21 AS final
LABEL stage=minimalbuilder

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/application /app/

RUN adduser -D -s /sbin/nologin app-user
RUN passwd -l root
RUN chown -R app-user:app-user /app
RUN chmod -R 500 /app

USER app-user

ENV PORT=8000

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --spider --quiet http://localhost:${PORT}/health || exit 1

ENTRYPOINT ["/app/application"]
