FROM golang:1.22.0 as build

WORKDIR /app

RUN go install github.com/abice/go-enum@latest

COPY go.mod .
COPY go.sum .

RUN go mod tidy
RUN go mod download

COPY . .

RUN go generate ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -ldflags="-w -s" -o bin/shoplanner cmd/shoplanner/main.go

FROM scratch as prod
COPY --from=build /app/bin/shoplanner /bin/shoplanner
ENTRYPOINT ["/bin/shoplanner"]