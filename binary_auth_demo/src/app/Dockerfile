FROM golang:1.8 as build

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN CGO_ENABLED=0 GOOS=linux go install -v ./...

FROM alpine
#FROM debian
COPY --from=build /go/bin/app /app

CMD ["/app"]