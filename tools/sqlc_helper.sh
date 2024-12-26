#!/bin/env bash

cp "$PROJECT_ROOT"/config/sqlc.yaml .
sqlc generate
rm sqlc.yaml
