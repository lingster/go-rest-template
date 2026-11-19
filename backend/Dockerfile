# syntax=docker/dockerfile:1
# Base images are pinned by digest (multi-arch index) for reproducible, tamper-evident builds;
# Dependabot (docker ecosystem) proposes digest bumps.
FROM golang:1.26.11-alpine@sha256:ecb67f987aa1a57794c91d19ade7ae0209f2f0f5496aaef9e6ae85ddf92d02ab AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux
COPY go.mod go.sum ./
# Verify module checksums against go.sum before compiling anything.
RUN --mount=type=cache,target=/go/pkg/mod go mod download && go mod verify
COPY . .
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -buildvcs=false -ldflags "-s -w -X main.version=${VERSION}" -o /out/server . \
 && mkdir -p /out/data

# Static binary with embedded swagger/views/static: no shell, no package manager, non-root.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/server /server
COPY --from=build --chown=65532:65532 /out/data /data
ENV HTTP_PORT=8080 DB_DRIVER=sqlite DATABASE_URL=file:/data/app.db
EXPOSE 8080
USER 65532:65532
# The binary probes its own /healthz (the image has no shell or curl).
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/server", "healthcheck"]
ENTRYPOINT ["/server"]
