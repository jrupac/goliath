# syntax=docker/dockerfile:1

FROM oven/bun AS frontend_builder

ENV NODE_ENV development

WORKDIR /frontend

EXPOSE 3000

RUN echo "Starting Goliath frontend (dev)..."
CMD ["bun", "run", "start"]