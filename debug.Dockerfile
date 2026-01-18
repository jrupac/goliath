# syntax=docker/dockerfile:1

FROM golang:1.24 AS backend_builder_debug

RUN echo "Installing deps..."
RUN DEBIAN_FRONTEND=noninteractive \
    apt-get update \
    && apt-get install --no-install-recommends --assume-yes \
    # For downloading pre-built protoc plugins
    curl \
    # To pull the delve binary \
    git \
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

ARG BUILD_TIMESTAMP=0
ARG BUILD_HASH=debug

RUN echo "Building Goliath core with debug flags..."
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -v -gcflags="all=-N -l" -ldflags  \
    "-X main.buildTimestamp=${BUILD_TIMESTAMP} \
    -X main.buildHash=${BUILD_HASH}" \
    -o goliath

RUN echo "Installing delve..."
WORKDIR /go/src/
RUN CGO_ENABLED=0 GOOS=linux go install github.com/go-delve/delve/cmd/dlv@latest

# Final development image
FROM scratch

COPY --from=backend_builder_debug /goliath /
COPY --from=backend_builder_debug /etc/ssl/certs /etc/ssl/certs/
COPY --from=backend_builder_debug /go/bin/dlv /

CMD ["/dlv", "--listen=:40000", "--headless=true", "--api-version=2", "exec", \
    "/goliath", \
    "--", \
    "--config=/config.ini"]
