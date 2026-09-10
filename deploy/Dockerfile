FROM --platform=$BUILDPLATFORM golang:1.26.8-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS build
ARG TARGETOS
ARG TARGETARCH
ARG REVISION=development
WORKDIR /src
COPY go.mod ./
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/catalog/ web/catalog/
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w -X main.build=$REVISION" -o /app/ ./cmd/artweb ./cmd/artrender ./cmd/artctl ./cmd/staticart
RUN mkdir -p /runtime/run/art /runtime/var/cache/art && \
    chown 10002:10000 /runtime/run/art && chmod 0770 /runtime/run/art && \
    chown 10001:10000 /runtime/var/cache/art && chmod 0700 /runtime/var/cache/art

FROM scratch AS app
ARG REVISION=development
LABEL org.opencontainers.image.revision=$REVISION
COPY --from=build /app/ /app/
COPY --from=build /runtime/ /
USER 10001:10000
ENTRYPOINT ["/app/artweb"]

FROM caddy:2.11.4-alpine@sha256:5f5c8640aae01df9654968d946d8f1a56c497f1dd5c5cda4cf95ab7c14d58648 AS edge
ARG REVISION=development
LABEL org.opencontainers.image.revision=$REVISION
COPY deploy/caddy/Caddyfile /etc/caddy/Caddyfile
RUN setcap -r /usr/bin/caddy && chown -R 10003:10003 /data /config
USER 10003:10003
