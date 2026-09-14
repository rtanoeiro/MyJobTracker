#!/bin/bash
## This script should only be used in Dev.
## It's used to seed dev data for interacting with the website

OPTION=$1
source .env

if [ "$OPTION" == "up" ]; then
    docker exec postgresql-job-applications psql -U $POSTGRES_USER -d $POSTGRES_DB -f app/internal/db/seed/001_seed.sql
elif [ "$OPTION" == "down" ]; then
    docker exec postgresql-job-applications psql -U $POSTGRES_USER -d $POSTGRES_DB -f app/internal/db/seed/002_remove_seed.sql
else
    echo "Invalid option"
    exit 1
fi