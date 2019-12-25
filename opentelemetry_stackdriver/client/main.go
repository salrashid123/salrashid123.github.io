package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"context"

	"cloud.google.com/go/storage"
	"golang.org/x/net/http2"

	"google.golang.org/api/iterator"

	"go.opentelemetry.io/otel/api/distributedcontext"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/key"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/exporter/trace/stackdriver"
	"go.opentelemetry.io/otel/plugin/httptrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var (
	httpport = flag.String("httpport", ":8081", "httpport")

	mytracer trace.Tracer
	fooKey   = key.New("ex.com/foo")
)

const (
)

type server struct{}

func backhandler(w http.ResponseWriter, r *http.Request) {
	log.Println("/backend called")

	bucketName := os.Getenv("OT_BUCKET")

	attrs, entries, spanCtx := httptrace.Extract(r.Context(), r)
	fmt.Println("extracted context")

	r = r.WithContext(distributedcontext.WithMap(r.Context(), distributedcontext.NewMap(distributedcontext.MapUpdate{
		MultiKV: entries,
	})))
	fmt.Println("request context modified")
	sctx, span := mytracer.Start(
		r.Context(),
		"Child-Trace",
		trace.WithAttributes(attrs...),
		trace.ChildOf(spanCtx),
	)

	err := mytracer.WithSpan(sctx, "child-operation", func(ctx context.Context) error {
		trace.CurrentSpan(ctx).SetAttributes(fooKey.String("no"))
		trace.CurrentSpan(ctx).AddEvent(ctx, "Event2", key.New("bogons").Int(100))

		time.Sleep(2 * time.Second)
		delay := rand.Intn(300) + 50
		time.Sleep(time.Duration(delay) * time.Millisecond)

		client, err := storage.NewClient(ctx)
		if err != nil {
			log.Fatal(err)
		}

		it := client.Bucket(bucketName).Objects(ctx, nil)
		for {
			attrs, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return err
			}
			log.Println(attrs.Name)
		}

		return nil
	})
	if err != nil {
		log.Fatalf("Error making request %v", err)
	}

	defer span.End()
	fmt.Println("worker done ")
	_, _ = io.WriteString(w, "work-done\n")
	return

	fmt.Fprint(w, "ok")
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

	http.HandleFunc("/backend", backhandler)
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
