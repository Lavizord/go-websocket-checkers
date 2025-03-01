#!/bin/bash

# Check if branch is passed as an argument
if [ -z "$1" ]; then
  echo "Usage: $0 <branch>"
  exit 1
fi

BRANCH=$1

# Fetch and pull the specified branch
echo "Fetching and pulling branch: $BRANCH"
git fetch origin $BRANCH
git checkout $BRANCH
git pull origin $BRANCH

# Stop all Docker containers
echo "Stopping all Docker containers..."
docker stop $(docker ps -aq)

# Remove all Docker containers
echo "Removing all Docker containers..."
docker rm $(docker ps -aq)

# Remove all Docker images
echo "Removing all Docker images..."
docker rmi -f $(docker images -q)

# Build and run Docker Compose
echo "Running Docker Compose with build..."
docker compose up --build -d

echo "Operation completed successfully."