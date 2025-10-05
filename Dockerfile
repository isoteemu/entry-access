# NOTICE: When updating base images, make sure they use the same base image (i.e. debian bookworm)
ARG GO_VERSION=1.25
ARG DEBIAN_VERSION=bookworm
ARG TAILWIND_VERSION=4.1.13

# Directory for build artifacts
ARG DIST_DIR=dist

FROM node:22-${DEBIAN_VERSION} AS assets-builder

ARG TARGETOS
ARG TARGETARCH
ARG TAILWIND_VERSION
ARG DIST_DIR

WORKDIR /app

# Map Docker arch to Tailwind arch and download binary
RUN set -eux; \
    if [ "$TARGETARCH" = "amd64" ]; then TAILWIND_ARCH="x64"; \
    elif [ "$TARGETARCH" = "arm64" ]; then TAILWIND_ARCH="arm64"; \
    else echo "Unsupported architecture: $TARGETARCH" && exit 1; fi; \
    curl -L "https://github.com/tailwindlabs/tailwindcss/releases/download/v${TAILWIND_VERSION}/tailwindcss-${TARGETOS}-${TAILWIND_ARCH}" \
        -o /usr/local/bin/tailwindcss && \
    chmod +x /usr/local/bin/tailwindcss

#COPY templates ./templates
COPY Makefile ./
COPY web/ ./web

RUN make assets DIST_DIR="${DIST_DIR}"

FROM golang:${GO_VERSION}-${DEBIAN_VERSION} AS builder

ARG DIST_DIR

# Create and change to the app directory.
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy compiled CSS from asset builder stage
COPY --from=assets-builder /app/${DIST_DIR} ${DIST_DIR}

# Use golang devcontainer

FROM mcr.microsoft.com/devcontainers/go:${GO_VERSION}-${DEBIAN_VERSION} AS dev

ARG DIST_DIR

WORKDIR /app

# Default anonymous volume for compiled stuff
VOLUME [ "/app/${DIST_DIR}" ]

# Copy compiled CSS from tailwind stage
COPY --from=assets-builder /app/${DIST_DIR} ${DIST_DIR}

COPY --from=assets-builder --chown=1000:1000 /usr/local/bin/tailwindcss /usr/local/bin/tailwindcss
COPY --from=builder --chown=1000:1000 /go /go
COPY --from=builder --chown=1000:1000 /app /app
