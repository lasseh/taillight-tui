# Builds the taillight-wish SSH server.
#
#   docker build -t taillight-wish .
#
FROM golang:alpine AS build
RUN apk add --no-cache git
WORKDIR /src

# Resolve modules first for better layer caching.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=${VERSION}" -o /taillight-wish ./cmd/taillight-wish

FROM alpine:latest
RUN adduser -D -u 65532 nonroot
USER nonroot
COPY --from=build /taillight-wish /taillight-wish
# Default SSH listen port (override with --listen).
EXPOSE 2222
ENTRYPOINT ["/taillight-wish"]
