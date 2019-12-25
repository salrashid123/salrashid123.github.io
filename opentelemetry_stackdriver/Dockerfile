FROM golang:1.13 AS build
WORKDIR /src/main
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go install -a -tags netgo -ldflags=-w

FROM gcr.io/distroless/base
COPY --from=build /go/bin/main /bin/main
EXPOSE 8080
ENTRYPOINT [ "/bin/main" ]