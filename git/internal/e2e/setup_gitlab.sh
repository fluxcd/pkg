#!/bin/bash

set -o errexit
PROJECT_DIR=$(git rev-parse --show-toplevel)

# Launch a container running GitLab CE.
docker run --detach \
  --hostname 127.0.0.1 \
  --publish 8080:80 --publish 2222:22 \
  --name gitlab \
  --shm-size 256m \
  gitlab/gitlab-ce:15.0.0-ce.0

# Wait for container to be healthy.
ok=false
retries=30
count=0
until ${ok}; do
    status=$(docker inspect gitlab -f '{{.State.Health.Status}}')
    if [[ "$status" = "healthy" ]]; then
        ok=true
    fi
    sleep 10
    count=$(($count + 1))
    if [[ ${count} -eq ${retries} ]]; then
        echo "Timed out waiting for GitLab container to be healthy"
        exit 1
    fi
done
echo "GitLab container is healthy"

# Grab the root password
password=$(docker exec gitlab grep 'Password:' /etc/gitlab/initial_root_password | sed "s/Password: //g")

# Register a PAT for the root user.
TOKEN="flux-gitlab-testing123"
docker cp $PROJECT_DIR/git/internal/e2e/setup_gitlab_pat.rb gitlab:/
docker exec gitlab gitlab-rails runner /setup_gitlab_pat.rb "${TOKEN}"
exitCode=$?
if [[ ${exitCode} -ne 0 ]]; then
    echo "Error while setting up GitLab PAT"
    exit ${exitCode}
fi
echo "GitLab PAT created successfully"

export GITLAB_PAT="${TOKEN}"
export GITLAB_ROOT_PASSWORD="${password}"
