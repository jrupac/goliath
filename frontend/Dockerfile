FROM node:lts AS frontend_builder

WORKDIR /

RUN echo "Populating dependencies..."
COPY package.json package-lock.json /
RUN npm i

RUN echo "Copying sources..."
COPY . .

RUN echo "Building Goliath frontend..."
RUN npm run build

# FROM gcr.io/distroless/base-debian11
# FROM docker.io/golang:1.21