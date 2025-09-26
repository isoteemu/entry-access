# NOTICE: When updating base images, make sure they use the same base image (i.e. debian bookworm)
ARG GO_VERSION=1.24
ARG DEBIAN_VERSION=bookworm
ARG TAILWIND_VERSION=4.1.13

FROM node:22-${DEBIAN_VERSION} AS tailwind

ARG TARGETOS
ARG TARGETARCH
ARG TAILWIND_VERSION

WORKDIR /app

# Map Docker arch to Tailwind arch and download binary
RUN set -eux; \
    if [ "$TARGETARCH" = "amd64" ]; then TAILWIND_ARCH="x64"; \
    elif [ "$TARGETARCH" = "arm64" ]; then TAILWIND_ARCH="arm64"; \
    else echo "Unsupported architecture: $TARGETARCH" && exit 1; fi; \
    curl -L "https://github.com/tailwindlabs/tailwindcss/releases/download/v${TAILWIND_VERSION}/tailwindcss-${TARGETOS}-${TAILWIND_ARCH}" \
        -o /usr/local/bin/tailwindcss && \
    chmod +x /usr/local/bin/tailwindcss

# Copy your project files
# COPY package.json tailwind.config.js ./
COPY assets ./assets
COPY templates ./templates

# Add fonts, CORS prevents hot-linking.
ADD https://www.jyu.fi/themes/custom/jyu/fonts/aleo/Aleo-Regular.otf ./assets/fonts/Aleo-Regular.otf
ADD https://www.jyu.fi/themes/custom/jyu/fonts/aleo/Aleo-Bold.otf ./assets/fonts/Aleo-Bold.otf
ADD https://www.jyu.fi/themes/custom/jyu/fonts/Lato/Lato-Regular.ttf ./assets/fonts/Lato-Regular.ttf
ADD https://www.jyu.fi/themes/custom/jyu/fonts/Lato/Lato-Black.ttf ./assets/fonts/Lato-Black.ttf
ADD https://www.jyu.fi/themes/custom/jyu/fonts/Lato/Lato-Bold.ttf ./assets/fonts/Lato-Bold.ttf

# Compile CSS.
RUN /usr/local/bin/tailwindcss -i ./assets/css/input.css -o ./assets/css/output.css

FROM golang:${GO_VERSION}-${DEBIAN_VERSION} AS builder

# Create and change to the app directory.
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy compiled CSS from tailwind stage
COPY --from=tailwind /app/assets /app/assets

# Use golang devcontainer

FROM mcr.microsoft.com/devcontainers/go:${GO_VERSION}-${DEBIAN_VERSION} AS dev

COPY --from=tailwind --chown=1000:1000 /usr/local/bin/tailwindcss /usr/local/bin/tailwindcss
COPY --from=builder --chown=1000:1000 /go /go
COPY --from=builder --chown=1000:1000 /app /app
