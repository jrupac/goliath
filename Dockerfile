# syntax=docker/dockerfile:1

# Build backend

FROM docker.io/golang:1.24 AS backend_builder_prod

RUN echo "Installing deps..."
RUN DEBIAN_FRONTEND=noninteractive \
    apt-get update \
    && apt-get install --no-install-recommends --assume-yes \
    # For downloading pre-built protoc plugins
    curl \
    # protoc binary
    protobuf-compiler \
    # Needed to validate SSL/TLS certificates
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

WORKDIR /

RUN echo "Copying sources..."
COPY backend/go.mod backend/go.sum /
RUN go mod download
COPY backend/ /

RUN echo "Building protobufs..."
RUN protoc --proto_path=admin/ --go_out=admin/ --go-grpc_out=admin/ \
    --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative \
    admin.proto

ARG BUILD_TIMESTAMP
ARG BUILD_HASH

RUN echo "Building Goliath core..."
RUN CGO_ENABLED=0 GOOS=linux go build -v -ldflags  \
    "-X main.buildTimestamp=${BUILD_TIMESTAMP} \
    -X main.buildHash=${BUILD_HASH}" \
    -o goliath

# Build frontend

FROM oven/bun AS frontend_builder_prod

WORKDIR /app

RUN echo "Populating dependencies..."
COPY frontend/package.json frontend/bun.lock ./
RUN bun install --frozen-lockfile

RUN echo "Copying sources..."
COPY frontend/ .

RUN echo "Building Goliath frontend..."
ENV NODE_ENV production
RUN bun run build

# Final deployment image

# Uncomment for image with more tools like a shell for debugging.
# FROM gcr.io/distroless/base-debian11
FROM scratch

COPY --from=backend_builder_prod /goliath /
COPY --from=backend_builder_prod /etc/ssl/certs /etc/ssl/certs/

COPY --from=frontend_builder_prod /app/build /public

CMD ["/goliath", "--config=/config.ini"]
