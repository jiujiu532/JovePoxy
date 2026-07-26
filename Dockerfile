# Multi-stage build for jovepoxy. Host agents must not install Docker.
FROM node:22-bookworm AS web
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:1.25-bookworm AS go-build
ARG VERSION=0.0.1
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -ldflags "-X jovepoxy/internal/version.Current=${VERSION}" -o /out/jovepoxy ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=go-build /out/jovepoxy /jovepoxy
ENV LISTEN=0.0.0.0:6446 DATA_DIR=/data
EXPOSE 6446
VOLUME ["/data"]
USER nonroot:nonroot
ENTRYPOINT ["/jovepoxy"]
