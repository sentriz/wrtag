# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.24-alpine3.21 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN  \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/ ./cmd/...

FROM alpine:3.20
LABEL org.opencontainers.image.source=https://github.com/sentriz/wrtag
RUN apk add -U --no-cache \
    rsgain
COPY --from=builder /out/* /usr/local/bin/
CMD ["wrtagweb"]
