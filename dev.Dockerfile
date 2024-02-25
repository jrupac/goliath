# syntax=docker/dockerfile:1

# Build backend

FROM golang:1.21 AS backend_builder

ENV CGO_ENABLED 0
ENV GOOS linux

RUN echo "Installing deps..."
RUN DEBIAN_FRONTEND=noninteractive \
    apt-get update \
    && apt-get install --no-install-recommends --assume-yes \
    # To pull the delve binary \
    git \
    # protoc binary
    protobuf-compiler \
    # Needed to validate SSL/TLS certificates
    ca-certificates

WORKDIR /

RUN echo "Copying sources..."
COPY backend/go.mod backend/go.sum /
RUN go mod download
COPY backend/ /

RUN echo "Building Goliath core with debug flags..."
RUN go build -x -v -mod=mod -gcflags="all=-N -l" -o goliath

# Get Delve from a GOPATH not from a Go Modules project
WORKDIR /go/src/
RUN go install github.com/go-delve/delve/cmd/dlv@latest

# Final development image
FROM scratch

COPY --from=backend_builder /goliath /
COPY --from=backend_builder /etc/ssl/certs /etc/ssl/certs/

COPY --from=backend_builder /go/bin/dlv /

CMD ["/dlv", "--listen=:40000", "--headless=true", "--api-version=2", "exec", \
    "/goliath", \
    "--", \
    "--config=/config.ini", "--logtostderr"]