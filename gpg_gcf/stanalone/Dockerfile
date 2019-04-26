FROM golang:1.11 as build

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN CGO_ENABLED=0 GOOS=linux go install -v ./...


FROM gcr.io/distroless/static
COPY --from=build /go/bin/app /app

EXPOSE 8080

CMD ["/app"]