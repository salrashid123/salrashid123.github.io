FROM golang:1.12 as build

RUN apt-get update -y && apt-get install -y libmagickwand-dev

WORKDIR /

COPY . .

RUN go get cloud.google.com/go/storage \
    gopkg.in/gographics/imagick.v2/imagick \
    contrib.go.opencensus.io/exporter/stackdriver \
    github.com/gorilla/mux

RUN go build src/main.go

EXPOSE 8080
ENTRYPOINT ["/main"]

