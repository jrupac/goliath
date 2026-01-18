# syntax=docker/dockerfile:1

FROM golang:1.24 AS cli_builder

RUN echo "Installing deps..."
RUN DEBIAN_FRONTEND=noninteractive \
    apt-get update \
    && apt-get install --no-install-recommends --assume-yes \
    # For downloading pre-built protoc plugins
    curl \
    # protoc binary
    protobuf-compiler

# Download pre-built protoc plugins
ARG PROTOC_GEN_GO_VERSION=1.36.6
ARG PROTOC_GEN_GO_GRPC_VERSION=1.5.1
RUN curl -sSL \
    "https://github.com/protocolbuffers/protobuf-go/releases/download/v${PROTOC_GEN_GO_VERSION}/protoc-gen-go.v${PROTOC_GEN_GO_VERSION}.linux.amd64.tar.gz" \
        | tar -xz -C /usr/local/bin
RUN curl -sSL \
    "https://github.com/grpc/grpc-go/releases/download/cmd/protoc-gen-go-grpc/v${PROTOC_GEN_GO_GRPC_VERSION}/protoc-gen-go-grpc.v${PROTOC_GEN_GO_GRPC_VERSION}.linux.amd64.tar.gz" \
        | tar -xz -C /usr/local/bin

WORKDIR /build

# Copy backend sources (needed for replace directive and proto compilation)
RUN echo "Copying backend sources..."
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend/ ./backend/

# Build protobufs
RUN echo "Building protobufs..."
RUN protoc --proto_path=backend/admin/ --go_out=backend/admin/ --go-grpc_out=backend/admin/ \
    --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative \
    admin.proto

# Copy CLI sources
RUN echo "Copying CLI sources..."
COPY cmd/goliath-cli/go.mod cmd/goliath-cli/go.sum ./cmd/goliath-cli/
RUN cd cmd/goliath-cli && go mod download
COPY cmd/goliath-cli/ ./cmd/goliath-cli/

# Build the CLI
RUN echo "Building goliath-cli..."
RUN cd cmd/goliath-cli && CGO_ENABLED=0 GOOS=linux go build -o /goliath-cli .

# Output stage - just the binary
FROM scratch AS export
COPY --from=cli_builder /goliath-cli /goliath-cli
