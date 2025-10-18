# syntax=docker/dockerfile:1

FROM golang:1.24 AS backend_builder_dev

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

RUN echo "Building protobufs..."
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
RUN export PATH="$PATH:$(go env GOPATH)/bin"
RUN protoc --proto_path=admin/ --go_out=admin/ --go-grpc_out=admin/ \
    --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative \
    admin.proto

RUN echo "Building Goliath core with debug flags..."
RUN CGO_ENABLED=0 GOOS=linux go build -v -gcflags="all=-N -l" -o goliath

# Final development image
FROM scratch

COPY --from=backend_builder_dev /goliath /
COPY --from=backend_builder_dev /etc/ssl/certs /etc/ssl/certs/

CMD ["/goliath", "--config=/config.ini"]
