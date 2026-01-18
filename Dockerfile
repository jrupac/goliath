# syntax=docker/dockerfile:1

# =============================================================================
# Shared base stage: Go + protoc tools
# =============================================================================
FROM golang:1.24 AS go_base

RUN DEBIAN_FRONTEND=noninteractive \
    apt-get update \
    && apt-get install --no-install-recommends --assume-yes \
    curl \
    git \
    protobuf-compiler \
    ca-certificates

# Download pre-built protoc plugins
ARG PROTOC_GEN_GO_VERSION=1.36.6
ARG PROTOC_GEN_GO_GRPC_VERSION=1.5.1
RUN curl -sSL \
    "https://github.com/protocolbuffers/protobuf-go/releases/download/v${PROTOC_GEN_GO_VERSION}/protoc-gen-go.v${PROTOC_GEN_GO_VERSION}.linux.amd64.tar.gz" \
        | tar -xz -C /usr/local/bin
RUN curl -sSL \
    "https://github.com/grpc/grpc-go/releases/download/cmd/protoc-gen-go-grpc/v${PROTOC_GEN_GO_GRPC_VERSION}/protoc-gen-go-grpc.v${PROTOC_GEN_GO_GRPC_VERSION}.linux.amd64.tar.gz" \
        | tar -xz -C /usr/local/bin

# =============================================================================
# Backend with compiled protobufs
# =============================================================================
FROM go_base AS backend_source

WORKDIR /

COPY backend/go.mod backend/go.sum /
RUN go mod download
COPY backend/ /

RUN protoc --proto_path=admin/ --go_out=admin/ --go-grpc_out=admin/ \
    --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative \
    admin.proto

# =============================================================================
# Backend build: production
# =============================================================================
FROM backend_source AS backend_prod

ARG BUILD_TIMESTAMP
ARG BUILD_HASH

RUN CGO_ENABLED=0 GOOS=linux go build -v -ldflags \
    "-X main.buildTimestamp=${BUILD_TIMESTAMP} \
    -X main.buildHash=${BUILD_HASH}" \
    -o goliath

# =============================================================================
# Backend build: development (with debug symbols)
# =============================================================================
FROM backend_source AS backend_dev

ARG BUILD_TIMESTAMP=0
ARG BUILD_HASH=dev

RUN CGO_ENABLED=0 GOOS=linux go build -v -gcflags="all=-N -l" -ldflags \
    "-X main.buildTimestamp=${BUILD_TIMESTAMP} \
    -X main.buildHash=${BUILD_HASH}" \
    -o goliath

# =============================================================================
# Backend build: debug (with debug symbols + delve)
# =============================================================================
FROM backend_source AS backend_debug

ARG BUILD_TIMESTAMP=0
ARG BUILD_HASH=debug

RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -v -gcflags="all=-N -l" -ldflags \
    "-X main.buildTimestamp=${BUILD_TIMESTAMP} \
    -X main.buildHash=${BUILD_HASH}" \
    -o goliath

WORKDIR /go/src/
RUN CGO_ENABLED=0 GOOS=linux go install github.com/go-delve/delve/cmd/dlv@latest

# =============================================================================
# CLI build
# =============================================================================
FROM go_base AS cli_builder

WORKDIR /build

# Copy backend sources (needed for replace directive and proto compilation)
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend/ ./backend/

RUN protoc --proto_path=backend/admin/ --go_out=backend/admin/ --go-grpc_out=backend/admin/ \
    --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative \
    admin.proto

# Copy and build CLI
COPY cmd/goliath-cli/go.mod cmd/goliath-cli/go.sum ./cmd/goliath-cli/
RUN cd cmd/goliath-cli && go mod download
COPY cmd/goliath-cli/ ./cmd/goliath-cli/

RUN cd cmd/goliath-cli && CGO_ENABLED=0 GOOS=linux go build -o /goliath-cli .

# =============================================================================
# Frontend build: production
# =============================================================================
FROM oven/bun AS frontend_builder

WORKDIR /app

COPY frontend/package.json frontend/bun.lock ./
RUN bun install --frozen-lockfile

COPY frontend/ .

ENV NODE_ENV=production
RUN bun run build

# =============================================================================
# Frontend: development (dev server, sources mounted as volume)
# =============================================================================
FROM oven/bun AS frontend-dev

ENV NODE_ENV=development
WORKDIR /frontend
EXPOSE 3000

CMD ["bun", "run", "start"]

# =============================================================================
# Final image: production
# =============================================================================
FROM scratch AS prod

COPY --from=backend_prod /goliath /
COPY --from=backend_prod /etc/ssl/certs /etc/ssl/certs/
COPY --from=frontend_builder /app/build /public

CMD ["/goliath", "--config=/config.ini"]

# =============================================================================
# Final image: development
# =============================================================================
FROM scratch AS dev

COPY --from=backend_dev /goliath /
COPY --from=backend_dev /etc/ssl/certs /etc/ssl/certs/

CMD ["/goliath", "--config=/config.ini"]

# =============================================================================
# Final image: debug (with delve)
# =============================================================================
FROM scratch AS debug

COPY --from=backend_debug /goliath /
COPY --from=backend_debug /etc/ssl/certs /etc/ssl/certs/
COPY --from=backend_debug /go/bin/dlv /

CMD ["/dlv", "--listen=:40000", "--headless=true", "--api-version=2", "exec", \
    "/goliath", \
    "--", \
    "--config=/config.ini"]

# =============================================================================
# CLI export
# =============================================================================
FROM scratch AS cli

COPY --from=cli_builder /goliath-cli /goliath-cli
