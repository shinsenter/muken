# syntax=docker/dockerfile:1

ARG ALPINE_VERSION=latest
ARG GO_VERSION=1.27

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
