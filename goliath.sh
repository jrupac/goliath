#!/usr/bin/env bash

# A script to build and package Goliath.

set -euo pipefail

CRDB_SRC_DIR="cockroach-data"
CRDB_VOL_NAME="crdb_volume"
CRDB_VOL_MOUNT="/cockroach-data"
CRDB_DEST_DIR="."
FORCE_VOL=false
DOCKER_COMPOSE_ARGS=()
BUILD_TIMESTAMP=$(date +%s)
BUILD_HASH=$(git rev-parse HEAD)
ENV=prod

# Export variables to make them visible during the build process
export BUILD_TIMESTAMP
export BUILD_HASH

function usage() {
 echo "Usage: $0 [OPTIONS]"
 echo "Options:"
 echo " -h, --help      Display this help message."
 echo " --crdb_dir DIR  CockroachDB data directory to populate volume. Ignored if volume exists unless --force_volume is set."
 echo " --env ENV       Environment to start (\"dev\" or \"prod\"). Default to \"prod\"."
 echo " --force_volume  Force creation of volume even if already exists (destroys existing volume)."
 echo " --force_build   Force a rebuild before starting containers. Dev environment is always rebuilt."
}

function handle_options() {
  while [ $# -gt 0 ]; do
    case $1 in
      -h | --help)
        usage
        exit 0
        ;;
      --crdb_dir)
        CRDB_SRC_DIR="$2"
        if [[ -z "${CRDB_SRC_DIR}" ]]; then
          echo "Value of crdb dir flag must be non-empty. Exiting."
          exit 1
        fi
        if [[ ! -d "${CRDB_SRC_DIR}" ]]; then
          echo "Directory specified by crdb dir flag does not exist: ${CRDB_SRC_DIR}. Exiting."
          exit 1
        fi
        shift
        ;;
      --env)
        ENV="$2"
        if [[ ! "${ENV}" =~ ^dev|prod$ ]]; then
          echo "--env must be set to \"dev\" or \"prod\", got: ${ENV}. Exiting."
          exit 1
        fi
        shift
        ;;
      --force_build)
        DOCKER_COMPOSE_ARGS+=("--build")
        ;;
      --force_volume)
        FORCE_VOL=true
        ;;
      *)
        echo "Invalid option: $1" >&2
        usage
        exit 1
        ;;
    esac
    shift
  done
}

# Inspired by https://stackoverflow.com/a/68511611.
function copy_to_volume() {
  SRC_PATH=$1
  VOLUME_MOUNT=$2
  DEST_PATH=$3

  # Create minimal, empty container.
  echo -e 'FROM scratch\nLABEL empty=""' | docker build -t empty -

  CONTAINER_ID=$(docker container create -v "${VOLUME_MOUNT}" empty cmd)
  docker cp "${SRC_PATH}/" "${CONTAINER_ID}:${DEST_PATH}"
  docker rm "${CONTAINER_ID}" --volumes
}

function setup_volume() {
  if [[ $FORCE_VOL = true ]]; then
    echo "Destroying existing CockroachDB data volume: ${CRDB_VOL_NAME} ..."

    # Make sure all volumes referenced in the Compose file are downed.
    docker compose down --volumes
    # Then make sure any stopped containers in the Compose file are also removed.
    docker compose rm --volumes
    docker volume rm -f "${CRDB_VOL_NAME}" > /dev/null
  fi

  if [[ -z $(docker volume ls -f name="${CRDB_VOL_NAME}" -q) ]]; then
    echo "Creating CockroachDB data volume..."

    docker volume create --name "${CRDB_VOL_NAME}" > /dev/null
    copy_to_volume \
        "${CRDB_SRC_DIR}" \
        "${CRDB_VOL_NAME}:${CRDB_VOL_MOUNT}" \
        "${CRDB_DEST_DIR}"
  fi
}

# Flag parsing.
handle_options "$@"

setup_volume

if [[ "${ENV}" == "dev" ]]; then
  echo "Starting dev Goliath..."
  docker compose --profile dev up --build
else
  echo "Starting production Goliath..."
  docker compose --profile prod up "${DOCKER_COMPOSE_ARGS[@]}"
fi
