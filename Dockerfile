FROM golang:1.25.5-alpine AS build

WORKDIR /app

RUN apk add --update --no-cache python3 && ln -sf python3 /usr/bin/python
RUN go install github.com/go-task/task/v3/cmd/task@v3.40.1

COPY taskfile.yml .

# RUN task deps

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

# RUN task generate
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 task build

FROM scratch AS prod
COPY --from=build /app/bin/backend /bin/shoplanner
COPY --from=build /app/config/backend.yml /etc/backend.yaml
ENTRYPOINT ["/bin/shoplanner"]
