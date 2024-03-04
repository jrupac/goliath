# syntax=docker/dockerfile:1

# Build backend

FROM docker.io/golang:1.21 AS backend_builder_prod

ENV CGO_ENABLED 0
ENV GOOS linux

RUN echo "Installing deps..."
RUN DEBIAN_FRONTEND=noninteractive \
    apt-get update \
    && apt-get install --no-install-recommends --assume-yes \
    # protoc binary
    protobuf-compiler \
    # Needed to validate SSL/TLS certificates
    ca-certificates

WORKDIR /

RUN echo "Copying sources..."
COPY backend/go.mod backend/go.sum /
RUN go mod download
COPY backend/ /

ARG BUILD_TIMESTAMP
ARG BUILD_HASH

RUN echo "Building Goliath core..."
RUN go build -x -v -mod=mod -ldflags  \
    "-X main.buildTimestamp=${BUILD_TIMESTAMP} \
    -X main.buildHash=${BUILD_HASH}" \
    -o goliath

# Build frontend

FROM node:lts AS frontend_builder_prod

ENV NODE_ENV production

WORKDIR /

RUN echo "Populating dependencies..."
COPY frontend/package.json frontend/package-lock.json /
RUN npm i

RUN echo "Copying sources..."
COPY frontend/ .

RUN echo "Building Goliath frontend..."
RUN npm run build

# Final deployment image

# Uncomment for image with more tools like a shell for debugging.
# FROM gcr.io/distroless/base-debian11
FROM scratch

COPY --from=backend_builder_prod /goliath /
COPY --from=backend_builder_prod /etc/ssl/certs /etc/ssl/certs/

COPY --from=frontend_builder_prod /build /public

CMD ["/goliath", "--config=/config.ini", "--logtostderr"]