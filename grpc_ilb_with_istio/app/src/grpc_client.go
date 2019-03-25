package main

import (

	pb "echo"
	"flag"
	"log"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	tlsCrt = "/data/certs/tls.crt"
	tlsKey = "/data/certs/tls.key"
)

var (
	conn *grpc.ClientConn
)

func main() {

	address := flag.String("host", "localhost:50051", "host:port of gRPC server")
	insecure := flag.Bool("insecure", false, "connect without TLS")
	flag.Parse()

	ce, err := credentials.NewClientTLSFromFile(tlsCrt, "")
	if err != nil {
		log.Fatalf("Failed to generate credentials %v", err)
	}

	if *insecure == true {
		conn, err = grpc.Dial(*address, grpc.WithInsecure())
	} else {
		conn, err = grpc.Dial(*address, grpc.WithTransportCredentials(ce))
	}
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	c := pb.NewEchoServerClient(conn)

	ctx := context.Background()

	for i := 0; i < 50; i++ {
		r, err := c.SayHello(ctx, &pb.EchoRequest{Name: "unary RPC msg "})
		if err != nil {
		  log.Printf("Server error:: %v", err)
		} else {
  		  log.Printf("RPC Response: %v %v", i, r)
		}	
	}

}
