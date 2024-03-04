# syntax=docker/dockerfile:1

# Build backend

FROM golang:1.21 AS backend_builder_dev

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

# Final development image
FROM scratch

COPY --from=backend_builder_dev /goliath /
COPY --from=backend_builder_dev /etc/ssl/certs /etc/ssl/certs/

CMD ["/goliath", "--config=/config.ini", "--logtostderr"]