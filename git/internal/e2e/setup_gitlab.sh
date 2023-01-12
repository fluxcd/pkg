#!/usr/bin/env bash

set -o errexit

IMAGE_REGISTRY=${REGISTRY:-}
IMAGE_REPO=gitlab
IMAGE_NAME=gitlab-ce
IMAGE_TAG=15.0.0-ce.0
CONTAINER_NAME=${GITLAB_CONTAINER:-gitlab-flux-e2e}
CONTAINER_IMAGE=${IMAGE_REPO}/${IMAGE_NAME}:${IMAGE_TAG}

# Update the container image with the provided registry address, if provided.
if [[ -n "${REGISTRY}" ]]; then
    CONTAINER_IMAGE=${IMAGE_REGISTRY}/${CONTAINER_IMAGE}
fi

PROJECT_DIR=$(git rev-parse --show-toplevel)

function create_gitlab_container {
    # Check and reuse any existing gitlab container.
    CONTAINER_ID=$(docker ps -f name=${CONTAINER_NAME} -q)
    if [[ -z "${CONTAINER_ID}" ]]; then
        echo "Creating gitlab container with image ${CONTAINER_IMAGE}..."
        docker run --detach \
        --hostname 127.0.0.1 \
        --publish 8080:80 --publish 2222:22 \
        --name ${CONTAINER_NAME} \
        --shm-size 256m \
        ${CONTAINER_IMAGE}
    else
        echo "Running tests against existing gitlab container ${CONTAINER_ID} ..."
    fi

    echo "Waiting for GitLab to be ready..."
    ok=false
    retries=30
    count=0
    until ${ok}; do
        status=$(docker inspect ${CONTAINER_NAME} -f '{{.State.Health.Status}}')
        if [[ "$status" = "healthy" ]]; then
            ok=true
        else
            sleep 10
        fi
        count=$(($count + 1))
        if [[ ${count} -eq ${retries} ]]; then
            echo "Timed out waiting for GitLab container to be healthy"
            exit 1
        fi
    done
    echo "GitLab container is healthy"
}

function copy_password {
    password=$(docker exec ${CONTAINER_NAME} grep 'Password:' /etc/gitlab/initial_root_password | sed "s/Password: //g")
    echo ${password}
}

create_gitlab_container

# Grab the root password
password="$(copy_password)"
if [[ ! "$password" ]]; then
    # The root passowrd file is deleted after 24 hours of Gitlab starting
    # https://docs.gitlab.com/ee/install/docker.html#installation
    echo "Root password no longer available, retrying with a new container..."
    docker stop ${CONTAINER_NAME} > /dev/null
    docker rm ${CONTAINER_NAME} > /dev/null
    create_gitlab_container
    password="$(copy_password)"
fi

# Register a PAT for the root user.
echo "Registering new PAT..."
TOKEN="flux-gitlab-testing-$RANDOM"
docker cp ${PROJECT_DIR}/git/internal/e2e/setup_gitlab_pat.rb ${CONTAINER_NAME}:/
docker exec ${CONTAINER_NAME} gitlab-rails runner /setup_gitlab_pat.rb "${TOKEN}"
exitCode=$?
if [[ ${exitCode} -ne 0 ]]; then
    echo "Error while setting up GitLab PAT"
    exit ${exitCode}
fi
echo "GitLab PAT created successfully"

export GITLAB_CE_PAT="${TOKEN}"
export GITLAB_ROOT_PASSWORD="${password}"
