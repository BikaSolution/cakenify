# Base images are pinned by tag AND digest so a re-pushed tag can't silently
# change what we build on. Dependabot (.github/dependabot.yml) bumps both.
FROM golang:1.27.4-alpine3.24@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /src

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOFLAGS=-mod=readonly \
    GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy only what the build needs (never `COPY . .`), so nothing stray in the
# build context — .env files, pgdata/, secrets — can end up in a layer.
# Templates and static assets are embedded via web/embed.go.
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY web ./web
RUN go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o /out/cakenify ./cmd/cakenify

# distroless/static: no shell, no package manager, no libc utilities — just
# CA certs and the (statically-linked, CGO_ENABLED=0) binary. The IANA tz
# database is embedded in the binary itself (time/tzdata import in main.go),
# so nothing else is needed for CAKENIFY_TIMEZONE to work.
# :nonroot runs as uid/gid 65532, never root.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329
COPY --from=build /out/cakenify /usr/local/bin/cakenify
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/cakenify"]
