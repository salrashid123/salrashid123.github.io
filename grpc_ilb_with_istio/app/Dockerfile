
FROM grpc/go as genpb
ADD . /go/
RUN protoc --go_out=plugins=grpc:. src/echo/echo.proto

FROM golang:1.10 as build
WORKDIR /go/
RUN go get golang.org/x/net/context \
        golang.org/x/net/http2 \
        google.golang.org/grpc \
        google.golang.org/grpc/credentials \
        google.golang.org/grpc/health \
        google.golang.org/grpc/health/grpc_health_v1 \
        google.golang.org/grpc/metadata
ADD . /go/
COPY --from=genpb /go/src/echo /go/src/echo
RUN export GOBIN=/go/bin && go install src/grpc_server.go
RUN export GOBIN=/go/bin && go install src/grpc_client.go

FROM gcr.io/distroless/base
COPY --from=build /go/bin /

EXPOSE 8081
EXPOSE 50051

#ENTRYPOINT ["grpc_server", "--grpcport", ":50051", "--httpport", ":8081"]
#ENTRYPOINT ["grpc_client", "--host",  "server.domain.com:50051"]
