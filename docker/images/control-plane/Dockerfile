FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/dockclawd ./cmd/dockclawd
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/dockclawctl ./cmd/dockclawctl

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget && \
    adduser -D -u 1000 zclaw

COPY --from=builder /out/dockclawd /usr/local/bin/dockclawd
COPY --from=builder /out/dockclawctl /usr/local/bin/dockclawctl

USER zclaw

EXPOSE 8080 8081

LABEL org.opencontainers.image.title="ZClaw Control Plane"
LABEL org.opencontainers.image.description="Docker-native headless agent platform"
LABEL org.opencontainers.image.source="https://github.com/zclaw/zclaw"

ENTRYPOINT ["/usr/local/bin/dockclawd"]
