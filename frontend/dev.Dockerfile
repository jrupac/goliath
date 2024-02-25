# syntax=docker/dockerfile:1

FROM node:lts AS frontend_builder

ENV NODE_ENV development

WORKDIR /frontend

EXPOSE 3000

RUN echo "Starting Goliath frontend (dev)..."
CMD ["npm", "run", "start"]

# FROM gcr.io/distroless/base-debian11
# FROM docker.io/golang:1.21