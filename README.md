# Build

Project can be launched locally or via Docker

Project is shipped with Taskfile build system

## Local build

```bash
# Install deps
task deps
# Launch codegenerating (enums, swagger, sqlc(?), mb mocks and grpc in future)
task generate
# Complile binaries
task build
```

## Docker
```bash
docker compose up -d
```

# Configuration

Note, that you need to create an appropriet env vars for this app.
Can be done ordinary or via .env file.

Example of .env file for given docker compose with default creds
```bash
DATABASE_PASSWORD=""
DATABASE_USER="root"
DATABASE_NAME="shoplanner"
DATABASE_NET="tcp"

REDIS_ADDR="redis:6379"
REDIS_PASS=""
REDIS_USER=""
REDIS_NET="tcp"

# use your own private key, this is example or for dev usage only
AUTH_PRIVATE_KEY="-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgcNWfMdC3QeMJlyS9\nH3xppu3gyjqZgeERTrBwyMAw6WGhRANCAAReuHw8bPa/vzs/1TJOwN3HDFRNa1DP\ng2gNyMq5z8S4/uMlS/zf1mfNIH1WZvkRVNIP3Iy1WS90rTyP/+rY+DYz\n-----END PRIVATE KEY-----\n"
```



