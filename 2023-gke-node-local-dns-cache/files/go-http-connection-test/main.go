package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/httptrace"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/gorilla/handlers"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func init() {
	// Some metrics have to be registered to be exposed
	prometheus.MustRegister(httpRequests)
}

var (
	Version   string
	Revision  string
	Branch    string
	BuildTime string
)

var nodeName = "unknown"

func main() {
	if os.Getenv("K8S_NODE_NAME") != "" {
		nodeName = os.Getenv("K8S_NODE_NAME")
	}

	fmt.Printf("Starting go-http-connection-test. Version: %s, Revision: %s, Branch: %s, BuildTime: %s. Running on node %s\n", Version, Revision, Branch, BuildTime, nodeName)

	buildTime, _ := time.Parse(time.RFC3339, BuildTime)

	buildInfoGauge.With(prometheus.Labels{"app": "go-http-connection-test", "version": Version, "revision": Revision, "branch": Branch, "buildtime": fmt.Sprintf("%v", buildTime.Unix())}).Set(1)
	buildTimeGauge.With(prometheus.Labels{"app": "go-http-connection-test", "version": Version, "revision": Revision, "branch": Branch}).Set(float64(buildTime.Unix()))

	// Get optional configurations
	listen := ":8080"
	if os.Getenv("LISTEN") != "" {
		listen = os.Getenv("LISTEN")
	}

	metricsListen := ":8088"
	if os.Getenv("METRICS_LISTEN") != "" {
		listen = os.Getenv("METRICS_LISTEN")
	}

	routesMountPath := "/"
	if os.Getenv("ROUTES_MOUNT_PATH") != "" {
		routesMountPath = os.Getenv("ROUTES_MOUNT_PATH")
	}

	var url1, url2 string
	var err error
	if os.Getenv("URL_1") != "" {
		url1 = os.Getenv("URL_1")
	} else {
		log.Fatal("Environment variable URL_1 must be set. Exiting.")
	}

	if os.Getenv("URL_2") != "" {
		url2 = os.Getenv("URL_2")
	}

	interval := time.Duration(time.Second * 15)
	if os.Getenv("INTERVAL") != "" {
		interval, err = time.ParseDuration(os.Getenv("INTERVAL"))
		if err != nil {
			log.Fatal(err)
		}
	}

	timeout := time.Duration(0)
	if os.Getenv("TIMEOUT") != "" {
		timeout, err = time.ParseDuration(os.Getenv("TIMEOUT"))
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("Configuration: INTERVAL: %v TIMEOUT: %v URL_1: %v URL_2: %v", interval, timeout, url1, url2)

	go func() {
		for {
			go doHttpRequest(url1, interval, timeout)
			time.Sleep(interval)
		}
	}()

	if url2 != "" {
		go func() {
			for {
				go doHttpRequest(url2, interval, timeout)
				time.Sleep(interval)
			}
		}()
	}

	http.Handle(fmt.Sprintf("%vhealth", routesMountPath), http.HandlerFunc(Health))

	// Start metrics server in the background
	go serveMetrics(metricsListen)

	// Start webserver
	log.Printf("Starting webserver on %v. Mounting routes on %v\n", listen, routesMountPath)
	log.Fatal(http.ListenAndServe(listen, handlers.LoggingHandler(os.Stdout, http.DefaultServeMux)))
}

func doHttpRequest(url string, interval time.Duration, timeout time.Duration) {
	uuid := uuid.New().String()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}

	t1 := time.Now()

	logtext := fmt.Sprintf("%v Making request to %v: ", uuid, url)

	reachedHTTPState := "notstarted"

	trace := &httptrace.ClientTrace{

		GetConn: func(hostPort string) {
			logtext = fmt.Sprintf("%v GetConn: %+v after %+v. ", logtext, hostPort, time.Since(t1))
			reachedHTTPState = "GetConn"
		},

		DNSStart: func(dnsStartInfo httptrace.DNSStartInfo) {
			logtext = fmt.Sprintf("%v DNSStart: %+v after %+v. ", logtext, dnsStartInfo, time.Since(t1))
			reachedHTTPState = "DNSStart"
		},

		DNSDone: func(dnsDoneInfo httptrace.DNSDoneInfo) {
			logtext = fmt.Sprintf("%v DNSDone: %+v after %+v. ", logtext, dnsDoneInfo, time.Since(t1))
			reachedHTTPState = "DNSDone"
		},

		ConnectStart: func(network, addr string) {
			logtext = fmt.Sprintf("%v ConnectStart: %+v , %v after %+v. ", logtext, network, addr, time.Since(t1))
			reachedHTTPState = "ConnectStart"
		},

		ConnectDone: func(network, addr string, err error) {
			logtext = fmt.Sprintf("%v ConnectDone: %+v , %v , %v after %+v. ", logtext, network, addr, err, time.Since(t1))
			reachedHTTPState = "ConnectDone"
		},

		TLSHandshakeStart: func() {
			logtext = fmt.Sprintf("%v TLSHandshakeStart: after %+v. ", logtext, time.Since(t1))
			reachedHTTPState = "TLSHandshakeStart"
		},

		TLSHandshakeDone: func(connState tls.ConnectionState, err error) {
			logtext = fmt.Sprintf("%v TLSHandshakeDone: %+v , %v after %+v. ", logtext, connState, err, time.Since(t1))
			reachedHTTPState = "TLSHandshakeDone"
		},

		GotConn: func(connInfo httptrace.GotConnInfo) {
			logtext = fmt.Sprintf("%v GotConn: %+v after %+v. ", logtext, connInfo, time.Since(t1))
			reachedHTTPState = "GotConn"
		},

		WroteHeaderField: func(key string, value []string) {
			logtext = fmt.Sprintf("%v WroteHeaderField: %+v - %v after %+v. ", logtext, key, value, time.Since(t1))
			reachedHTTPState = "WroteHeaderField"
		},

		WroteHeaders: func() {
			logtext = fmt.Sprintf("%v WroteHeaders: after %+v. ", logtext, time.Since(t1))
			reachedHTTPState = "WroteHeaders"
		},

		WroteRequest: func(wroteRequestInfo httptrace.WroteRequestInfo) {
			logtext = fmt.Sprintf("%v WroteRequest: %+v after %+v. ", logtext, wroteRequestInfo, time.Since(t1))
			reachedHTTPState = "WroteRequest"
		},

		GotFirstResponseByte: func() {
			logtext = fmt.Sprintf("%v GotFirstResponseByte: after %+v. ", logtext, time.Since(t1))
			reachedHTTPState = "GotFirstResponseByte"
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	// Disable connection reuse
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		}}

	var resp *http.Response

	resp, err = client.Do(req)

	if err != nil {
		logtext = fmt.Sprintf("%v ERROR HTTP request failed: %v after %+v. ", logtext, err, time.Since(t1))
	} else {

		if resp.StatusCode == http.StatusOK {
			reachedHTTPState = "Completed"
		} else {
			reachedHTTPState = "Non-200-Response"
			logtext = fmt.Sprintf("%v ERROR Non-OK HTTP status: %v. ", logtext, resp.StatusCode)
		}
		resp.Body.Close()

		logtext = fmt.Sprintf("%v Read response after %+v. ", logtext, time.Since(t1))
	}

	fmt.Printf("%v HTTP request reached state %+v. \n", logtext, reachedHTTPState)

	httpRequests.With(prometheus.Labels{"url": url, "http_state": reachedHTTPState}).Inc()
}

func serveMetrics(metricsListen string) {
	fmt.Printf("Starting metrics server on %v\n", metricsListen)
	http.Handle("/metrics", promhttp.Handler())

	log.Fatal(http.ListenAndServe(metricsListen, handlers.LoggingHandler(os.Stdout, http.DefaultServeMux)))
}

func Health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "{'status': 'OK', 'node': '%s'}", nodeName)
}

// Variables for storing metrics
var (
	buildInfoGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "signicat",
			Name:      "build_info",
			Help:      "Build info",
		},
		[]string{"app", "version", "revision", "branch", "buildtime"},
	)

	buildTimeGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "signicat",
			Name:      "build_time",
			Help:      "Build time",
		},
		[]string{"app", "version", "revision", "branch"},
	)

	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "signicat",
			Name:      "http_requests",
			Help:      "Number of HTTP requests and how far they got",
		},
		[]string{"url", "http_state"},
	)
)
