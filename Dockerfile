FROM golang:1.23.4 AS build

WORKDIR /app

ENV GOBIN=/bin

RUN go install github.com/go-task/task/v3/cmd/task@latest

COPY taskfile.yml .

RUN task deps

COPY go.mod .
COPY go.sum .

RUN go mod tidy
RUN go mod download

COPY . .

RUN task generate
RUN CGO_ENABLED=0 task build

FROM scratch AS prod
COPY --from=build /app/bin/backend /bin/shoplanner
COPY --from=build /app/config/backend.yml /etc/backend.yaml
ENTRYPOINT ["/bin/shoplanner"]
