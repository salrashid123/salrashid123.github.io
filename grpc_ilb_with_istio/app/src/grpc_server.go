package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"echo"

	"golang.org/x/net/context"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	tlsCrt = "/data/certs/tls.crt"
	tlsKey = "/data/certs/tls.key"

	serviceName = "echo.EchoServer"
	hcAddress = "127.0.0.1:9000"
)

var (
	grpcport = flag.String("grpcport", "", "grpcport")
	httpport = flag.String("httpport", ":8081", "httpport")
	insecure = flag.Bool("insecure", false, "startup without TLS")

	healthsrv *health.Server
)

type server struct{}

type hcserver struct {
	mu        sync.Mutex
	statusMap map[string]healthpb.HealthCheckResponse_ServingStatus
}

func (s *hcserver) SetServingStatus(service string, status healthpb.HealthCheckResponse_ServingStatus) {
	s.mu.Lock()
	s.statusMap[service] = status
	s.mu.Unlock()
}

func (s *server) SayHello(ctx context.Context, in *echo.EchoRequest) (*echo.EchoReply, error) {

	log.Println("Got rpc: --> ", in.Name)

	var h, err = os.Hostname()
	if err != nil {
		log.Printf("Unable to get hostname %v", err)
		healthsrv.SetServingStatus(serviceName, healthpb.HealthCheckResponse_NOT_SERVING)
		return nil, grpc.Errorf(codes.Internal, "Unable to get hostname ")
	}
	return &echo.EchoReply{Message: "Hello " + in.Name + "  from hostname " + h}, nil
}

func fronthandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "hello world")
}

func healthhandler(w http.ResponseWriter, r *http.Request) {

	ctx, _ := context.WithTimeout(context.Background(), time.Second * 1)
	conn, err := grpc.Dial(hcAddress, grpc.WithInsecure())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	defer conn.Close()
	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{Service: serviceName})
	if err != nil {
		log.Printf("Failed to get HealthCheck...could not make connection: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		log.Printf("Service unhealthy ", resp.GetStatus().String())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}

	fmt.Fprint(w, serviceName + ": "+  resp.GetStatus().String())
}

func main() {

	flag.Parse()

	if *grpcport == "" {
		fmt.Fprintln(os.Stderr, "missing -grpcport flag (:50051)")
		flag.Usage()
		os.Exit(2)
	}
	if *httpport == "" {
		fmt.Fprintln(os.Stderr, "missing -httpport flag, using defaults(:8080)")
	}

	http.HandleFunc("/", fronthandler)
	http.HandleFunc("/_ah/health", healthhandler)

	srv := &http.Server{
		Addr: *httpport,
	}
	http2.ConfigureServer(srv, &http2.Server{})

	if *insecure == true {
		go srv.ListenAndServe()
	} else {
		go srv.ListenAndServeTLS(tlsCrt, tlsKey)
	}

	ce, err := credentials.NewServerTLSFromFile(tlsCrt, tlsKey)
	if err != nil {
		log.Fatalf("Failed to generate credentials %v", err)
	}

	lis, err := net.Listen("tcp", *grpcport)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	sopts := []grpc.ServerOption{grpc.MaxConcurrentStreams(10)}
	if *insecure == false {
		sopts = append(sopts, grpc.Creds(ce))
	}
	s := grpc.NewServer(sopts...)

	echo.RegisterEchoServerServer(s, &server{})

	hs := grpc.NewServer()
	hlis, err := net.Listen("tcp", hcAddress)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	healthsrv = health.NewServer()
	healthpb.RegisterHealthServer(hs, healthsrv)
	go hs.Serve(hlis)

	// set ok status for our our service's health check...normally this is done after service init functions..
	healthsrv.SetServingStatus(serviceName, healthpb.HealthCheckResponse_SERVING)

	log.Printf("Starting gRPC server on port %v", *grpcport)

	s.Serve(lis)
}
