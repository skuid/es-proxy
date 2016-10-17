package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client/metadata"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
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

	director := func(req *http.Request) {
		req.URL.Scheme = "https"
		req.Host = *esDomain
		req.URL.Host = *esDomain
		req.Header.Set("Connection", "close")

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
		req.Body = ioutil.NopCloser(bytes.NewBuffer(bodyData))

		config := aws.NewConfig().WithCredentials(creds)
		config = config.WithRegion(*region)
		clientInfo := metadata.ClientInfo{
			ServiceName: "es",
		}
		operation := &request.Operation{
			Name:       "",
			HTTPMethod: req.Method,
			HTTPPath:   req.URL.Path,
		}
		handlers := request.Handlers{}
		awsReq := request.New(*config, clientInfo, handlers, nil, operation, nil, nil)
		awsReq.SetBufferBody(bodyData)
		awsReq.HTTPRequest.URL = req.URL
		awsReq.Sign()

		for k, v := range awsReq.HTTPRequest.Header {
			req.Header[k] = v
		}

		log.Debug(req.URL)
		for k, header := range req.Header {
			log.Debugf("    %v: %v", k, header)
		}
	}
	proxy := &httputil.ReverseProxy{Director: director}
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *listenPort), proxy))
}
