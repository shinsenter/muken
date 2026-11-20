# syntax=docker/dockerfile:1

ARG ALPINE_VERSION=latest
ARG GO_VERSION=1.27
# "latest-alpine" is the alpine variant of shinsenter/laravel (pulled on 2026-11-20).
ARG LARAVEL_IMAGE=shinsenter/laravel:latest-alpine

# The Go build runs on the machine that builds the image and cross-compiles,
# so an arm64 image needs no emulation for this step.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
COPY src/go.mod src/go.sum ./
RUN go mod download
COPY src/ .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o /out/muken ./cmd/muken

# The plain image: muken and an SSH client. It has no Docker CLI and no daemon,
# because muken runs every docker command on the server over SSH.
# "latest" and `apk upgrade` pick up the newest security fixes at each build.
FROM alpine:${ALPINE_VERSION} AS cli
LABEL org.opencontainers.image.source="https://github.com/shinsenter/muken" \
      org.opencontainers.image.description="Put your Docker app on a server you own, with no downtime" \
      org.opencontainers.image.licenses="Apache-2.0"
# ca-certificates: muken calls the DigitalOcean, Hetzner and Cloudflare APIs over HTTPS.
# openssh-client: `muken ssh`, `exec`, `logs -f` and `agent open` run the ssh command.
RUN apk upgrade --no-cache && apk add --no-cache ca-certificates openssh-client
COPY --from=build /out/muken /usr/local/bin/muken
COPY LICENSE NOTICE TRADEMARK.md THIRD_PARTY_NOTICES.txt /licenses/muken/
WORKDIR /work
ENTRYPOINT ["muken"]
CMD ["help"]

# The web app: a new Laravel project with Livewire and the files of web/ on top, and the assets built
# with Vite. The final stage copies only the result, so node stays out of the image.
# In this image `php`, `composer` and `artisan` are aliases that run in APP_PATH, so the build stages
# set APP_PATH to the folder they work in. The final stage keeps the default /var/www/html.
FROM ${LARAVEL_IMAGE} AS web-build
ENV APP_PATH=/work
WORKDIR /work
RUN apk add --no-cache nodejs npm \
    && composer create-project laravel/laravel . --no-interaction --prefer-dist \
    && composer require livewire/livewire --no-interaction
COPY web/ /work/
# The default migrations (users, cache, jobs) are not used: the only table is the history.
# An empty APP_KEY makes init-laravel of the base image generate a key at the first start.
RUN rm -f database/migrations/*.php \
    && sed -i 's/^APP_KEY=.*/APP_KEY=/' .env \
    && npm install --no-audit --no-fund \
    && npm run build

# Tests of web/. Build this target on its own: `docker build -f src/Dockerfile --target web-test .`
# The key is empty in web-build on purpose, so the tests make their own.
FROM web-build AS web-test
RUN php artisan key:generate --force && php artisan test

# What the image ships: no tests, no node_modules, no dev packages.
FROM web-build AS web-app
RUN rm -rf tests node_modules \
    && composer install --no-dev --optimize-autoloader --no-interaction

# The web image: the site and the muken CLI in one image. SSH is on, and it needs SSHD_AUTHORIZED_KEYS.
# Docker needs no daemon here, because muken runs every docker command on the server over SSH.
FROM ${LARAVEL_IMAGE} AS web
LABEL org.opencontainers.image.source="https://github.com/shinsenter/muken" \
      org.opencontainers.image.description="muken: the CLI and a web UI to put your Docker app on a server you own" \
      org.opencontainers.image.licenses="Apache-2.0"
RUN apk upgrade --no-cache && apk add --no-cache ca-certificates openssh-client
COPY --from=build /out/muken /usr/local/bin/muken
COPY LICENSE NOTICE TRADEMARK.md THIRD_PARTY_NOTICES.txt /licenses/muken/
COPY --from=web-app --chown=www-data:www-data /work/ /var/www/html/
COPY src/docker/web/rootfs/ /
# Sessions, cache and queue use files, so the history is the only use of a database.
# LARAVEL_AUTO_MIGRATION is off because the app prepares its own SQLite file (see HistoryStore).
ENV ENABLE_SSHD=1 \
    APP_ENV=production \
    APP_DEBUG=false \
    LOG_CHANNEL=stderr \
    SESSION_DRIVER=file \
    CACHE_STORE=file \
    QUEUE_CONNECTION=sync \
    DB_CONNECTION=sqlite \
    LARAVEL_AUTO_MIGRATION=0 \
    LARAVEL_ENABLE_SCHEDULER=0 \
    MUKEN_WORKDIR=/work \
    MUKEN_CONFIG=/work/muken.yaml \
    MUKEN_API_ADDR=127.0.0.1:7600
