package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"net/http"
	"os"
	"time"

	"context"

	"golang.org/x/net/http2"
	"google.golang.org/grpc/codes"

	"go.opentelemetry.io/otel/api/distributedcontext"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/key"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/exporter/trace/stackdriver"
	"go.opentelemetry.io/otel/plugin/httptrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var (
	httpport = flag.String("httpport", ":8080", "httpport")

	mytracer trace.Tracer
	fooKey   = key.New("ex.com/foo")
)

const ()

type server struct{}

func fronthandler(w http.ResponseWriter, r *http.Request) {
	log.Println("/frontend called")
	client := http.DefaultClient
	ctx := distributedcontext.NewContext(r.Context(),
		key.String("username", "donuts"),
	)

	workerURL := "http://localhost:8081/backend"
	err := mytracer.WithSpan(ctx, "parent-tracer",
		func(ctx context.Context) error {
			err := mytracer.WithSpan(ctx, "operation", func(ctx context.Context) error {
				trace.CurrentSpan(ctx).AddEvent(ctx, "Event1", key.New("bogons").Int(100))
				trace.CurrentSpan(ctx).SetAttributes(fooKey.String("yes"))
				time.Sleep(2 * time.Second)
				return nil
			})

			req, _ := http.NewRequest("GET", workerURL, nil)

			ctx, req = httptrace.W3C(ctx, req)
			httptrace.Inject(ctx, req)

			res, err := client.Do(req)
			if err != nil {
				log.Fatalf("Unable to make request %v", err)
			}
			body, err := ioutil.ReadAll(res.Body)
			_ = res.Body.Close()
			log.Printf("Backend Response %s", string(body))
			trace.CurrentSpan(ctx).SetStatus(codes.OK)

			return err
		})

	if err != nil {
		log.Fatalf("Error making request %v", err)
	}

	fmt.Fprint(w, "ok from frontend")
}

func healthhandler(w http.ResponseWriter, r *http.Request) {
	log.Println("heathcheck...")
	fmt.Fprint(w, "ok from backend")
}

func main() {

	flag.Parse()

	if *httpport == "" {
		fmt.Fprintln(os.Stderr, "missing -httpport flag (:8080)")
		flag.Usage()
		os.Exit(2)
	}

	projectID := os.Getenv("PROJECT_ID")

	exporter, err := stackdriver.NewExporter(
		stackdriver.WithProjectID(projectID),
	)
	if err != nil {
		log.Fatal(err)
	}

	tp, err := sdktrace.NewProvider(sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithSyncer(exporter))
	if err != nil {
		log.Fatal(err)
	}
	global.SetTraceProvider(tp)

	mytracer = global.TraceProvider().Tracer("stackdriver/example/starter")

	http.HandleFunc("/frontend", fronthandler)
	http.HandleFunc("/_ah/health", healthhandler)

	srv := &http.Server{
		Addr: *httpport,
	}
	http2.ConfigureServer(srv, &http2.Server{})
	//err := srv.ListenAndServeTLS("server_crt.pem", "server_key.pem")
	err = http.ListenAndServe(*httpport, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
