# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/server . \
 && mkdir -p /out/data

# Static binary with embedded swagger/views/static: no files needed next to it.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
COPY --from=build --chown=65532:65532 /out/data /data
ENV HTTP_PORT=8080 DB_DRIVER=sqlite DATABASE_URL=file:/data/app.db
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
