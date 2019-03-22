package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/exporter/prometheus"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"go.opencensus.io/stats"
	//"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	//"go.opencensus.io/trace"
	//"go.opencensus.io/zpages"
	//"contrib.go.opencensus.io/exporter/stackdriver/propagation"
	"go.opencensus.io/plugin/ochttp"
)

var (
	mLatencyMs   = stats.Float64("latency", "The latency in milliseconds", "ms")
	keyMethod, _ = tag.NewKey("method")
	latencyView  = &view.View{
		Name:        "demo/Latency",
		Measure:     mLatencyMs,
		Description: "The distribution of latencies",
		// https://github.com/census-ecosystem/opencensus-go-exporter-stackdriver/issues/98
		//Aggregation: view.Distribution(0, 25, 50, 100, 250, 500, 1000, 2500, 5000),
		Aggregation: view.Distribution(25, 50, 100, 250, 500, 1000, 2500, 5000),		
		TagKeys:     []tag.Key{keyMethod},
	}
)


func printInfo(resp http.ResponseWriter, req *http.Request) {

	startTime := time.Now()
	defer func() {
		ms := float64(time.Since(startTime).Nanoseconds()) / 1e6
		ctx, err := tag.New(context.Background(), tag.Insert(keyMethod, "recordVisit"))
		if err != nil {
			log.Println(err)
		}
		log.Printf("Recording: %v\n", ms)
		stats.Record(ctx, mLatencyMs.M(ms))
	}()

	for _, e := range os.Environ() {
		fmt.Fprintf(resp, "%s \n", e)
	}
}

func hc(resp http.ResponseWriter, req *http.Request) {

	startTime := time.Now()
	defer func() {
		ms := float64(time.Since(startTime).Nanoseconds()) / 1e6
		ctx, err := tag.New(context.Background(), tag.Insert(keyMethod, "recordVisit"))
		if err != nil {
			log.Println(err)
		}
		log.Printf("Recording: %v\n", ms)
		stats.Record(ctx, mLatencyMs.M(ms))
	}()

	fmt.Fprintf(resp, "ok")
}

func makereq(resp http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	defer func() {
		ms := float64(time.Since(startTime).Nanoseconds()) / 1e6
		ctx, err := tag.New(context.Background(), tag.Insert(keyMethod, "recordVisit"))		
		if err != nil {
			log.Println(err)
		}
		log.Printf("Recording: %v\n", ms)
		stats.Record(ctx, mLatencyMs.M(ms))
	}()

	//client := &http.Client{}
	client := &http.Client{Transport: &ochttp.Transport{}}

	// start span
	c, sleepSpan := trace.StartSpan(req.Context(), "start=sleep_for_no_reason")
	time.Sleep(200 * time.Millisecond)
	sleepSpan.End()
	// end span

	// Start span
	c, span := trace.StartSpan(req.Context(), "start=requst_to_backend")

	hreq, _ := http.NewRequest("GET", "http://localhost:8080/backend", nil)
	// add context to outbound http request
	hreq = hreq.WithContext(c)
	rr, err := client.Do(hreq)
	if err != nil {
		log.Printf("Unable to print file contentt: %v", err)
		http.Error(resp, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	fmt.Fprintf(resp, "%s \n", rr.Status)
	rr.Body.Close()
	span.End()
	// end Span
}

func backend(resp http.ResponseWriter, req *http.Request) {

	// Acquire inbound context
	ctx := req.Context()

	tokenSource, err := google.DefaultTokenSource(oauth2.NoContext, storage.ScopeReadOnly)
	if err != nil {
		log.Printf("Unable to acquire token source: %v", err)
		http.Error(resp, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}

	// Make GCS api request
	storeageCient, err := storage.NewClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		log.Printf("Unable to acquire storage Client: %v", err)
		http.Error(resp, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}

	bkt := storeageCient.Bucket(os.Getenv("GOOGLE_CLOUD_PROJECT"))			
	obj := bkt.Object("some_file.txt")

	//r, err := obj.NewReader(context.Background())
	r, err := obj.NewReader(ctx)
	if err != nil {
		log.Printf("Unable to read filest: %v", err)
		http.Error(resp, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	defer r.Close()
	
	// End GCS API call
	
	// Start Span
	_, fileSpan := trace.StartSpan(ctx, "start=print_file")
	if _, err := io.Copy(os.Stdout, r); err != nil {
		log.Printf("Unable to print file contentt: %v", err)
		http.Error(resp, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	time.Sleep(50 * time.Millisecond)
	fileSpan.End()
	// End Span

	fmt.Fprintf(resp, "backend")
}

func main() {

	var wg sync.WaitGroup

	sd, err := stackdriver.NewExporter(stackdriver.Options{
		ProjectID:    os.Getenv("GOOGLE_CLOUD_PROJECT"),
		MetricPrefix: "demo",
	})
	if err != nil {
		log.Fatal(err)
	}
	trace.ApplyConfig(trace.Config{
		DefaultSampler: trace.AlwaysSample(),
	})
	trace.RegisterExporter(sd)

	if err := view.Register(latencyView); err != nil {
		log.Fatal(err)
	}
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "demo",
	})
	if err != nil {
		log.Fatalf("Failed to create Prometheus exporter: %v", err)
	}
	view.RegisterExporter(sd)
	view.RegisterExporter(pe)
	view.SetReportingPeriod(60 * time.Second)

	wg.Add(1)
	go func() {
		defer wg.Done()
		mux := http.NewServeMux()
		mux.Handle("/metrics", pe)
		log.Fatal(http.ListenAndServe(":9091", mux))
	}()

	http.HandleFunc("/", printInfo)
	http.HandleFunc("/_ah/health", hc)
	http.HandleFunc("/makereq", makereq)
	http.HandleFunc("/backend", backend)

	log.Fatal(http.ListenAndServe(":8080", &ochttp.Handler{
		IsPublicEndpoint: false,
	}))
	wg.Wait()
}
