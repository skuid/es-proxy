package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/private/protocol/rest"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stderr)

	level := os.Getenv("LOG_LEVEL")
	if len(level) == 0 {
		level = "info"
	}
	if lvl, err := log.ParseLevel(level); err != nil {
		log.Errorf("Level '%s' is invalid: falling back to INFO", level)
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(lvl)
	}
}

var esDomain = flag.String("domain", os.Getenv("ES_DOMAIN"), "The elasticsearch domain to proxy")
var listenPort = flag.Int("port", 8080, "Listening port for proxy")
var region = flag.String("region", os.Getenv("AWS_REGION"), "AWS region for credentials")

func main() {
	flag.Parse()

	log.Printf("Connected to %s", *esDomain)
	log.Printf("AWS ES cluster available at http://127.0.0.1:%d", *listenPort)
	log.Printf("Kibana available at http://127.0.0.1:%d/_plugin/kibana/", *listenPort)
	creds := credentials.NewEnvCredentials()

	if _, err := creds.Get(); err != nil {
		log.Fatalf("Failed to load credentials: %v", err)
	}
	signer := v4.NewSigner(creds)

	director := func(req *http.Request) {
		req.URL.Scheme = "https"
		req.Host = *esDomain
		req.URL.Host = *esDomain
		req.Header.Set("Connection", "close")

		if strings.Contains(req.URL.RawPath, "%2C") {
			req.URL.RawPath = rest.EscapePath(req.URL.RawPath, false)
		}

		log.WithFields(log.Fields{
			"method": req.Method,
			"path":   req.URL.Path,
		}).Debug()
		t := time.Now()
		req.Header.Set("Date", t.Format(time.RFC3339))

		bodyData, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.WithFields(log.Fields{
				"method": req.Method,
				"path":   req.URL.Path,
			}).Errorf("Failed to consume body %v", err)
			return
		}
		buf := bytes.NewReader(bodyData)

		if _, err := signer.Sign(req, buf, "es", *region, t); err != nil {
			log.WithFields(log.Fields{
				"method": req.Method,
				"path":   req.URL.Path,
			}).Errorf("Failed to sign request %v", err)
		}
	}
	proxy := &httputil.ReverseProxy{Director: director}
	log.Fatal(http.ListenAndServe(":9200", proxy))
}
