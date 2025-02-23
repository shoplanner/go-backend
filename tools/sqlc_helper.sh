#!/bin/env bash

cp "$PROJECT_ROOT"/config/sqlc.yaml .
go tool github.com/sqlc-dev/sqlc/cmd/sqlc generate
rm sqlc.yaml
