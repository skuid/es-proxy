package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	auth "github.com/skuid/go-middlewares/authn/google"
	"github.com/skuid/spec"
	"github.com/skuid/spec/lifecycle"
	_ "github.com/skuid/spec/metrics"
	"github.com/skuid/spec/middlewares"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func init() {
	l, err := spec.NewStandardLogger()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	zap.ReplaceGlobals(l)
	err = os.Setenv("AWS_SDK_LOAD_CONFIG", "true")
	if err != nil {
		zap.L().Fatal(err.Error())
	}
}

func main() {
	flag.String("domain", "", "The elasticsearch domain to proxy")
	flag.Int("port", 3000, "Listening port for proxy")
	flag.String("region", "us-west-2", "AWS region for credentials")
	flag.Bool("auth-enable", false, "enable Google OIDC authentication")
	flag.String("auth-email-domain", "", "allowed user domains")
	flag.Int("metrics-port", 3001, "management endpoint port")
	flag.Parse()

	viper.BindPFlags(flag.CommandLine)
	viper.SetEnvPrefix("esproxy")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	zap.L().Info("Connected to Elasticsearch",
		zap.String("domain", viper.GetString("domain")),
		zap.String("es_host", fmt.Sprintf("http://127.0.0.1:%d", viper.GetInt("port"))),
		zap.String("kibana_host", fmt.Sprintf("http://127.0.0.1:%d/_plugin/kibana/", viper.GetInt("port"))),
	)

	director := func(req *http.Request) {
		req.URL.Scheme = "https"
		req.Host = viper.GetString("domain")
		req.URL.Host = viper.GetString("domain")
		req.Header.Set("Connection", "close")

		t := time.Now()
		req.Header.Set("Date", t.Format(time.RFC3339))

		sess, err := session.NewSession(
			&aws.Config{CredentialsChainVerboseErrors: aws.Bool(true)},
		)
		if err != nil {
			zap.L().Error("Error creating AWS session", zap.Error(err))
			return
		}

		creds := sess.Config.Credentials
		if _, err := creds.Get(); err != nil {
			zap.L().Error("Failed to load credentials", zap.Error(err))
			return
		}
		signer := v4.NewSigner(creds)
		var bodyData []byte
		if req.Body != nil {
			bodyData, err = ioutil.ReadAll(req.Body)
			if err != nil {
				zap.L().Error(err.Error(), zap.String("method", req.Method), zap.String("path", req.URL.Path))
				return
			}
		}
		if _, err := signer.Sign(req, bytes.NewReader(bodyData), "es", viper.GetString("region"), t); err != nil {
			zap.L().Error("Error signing request", zap.Error(err))
			return
		}
	}
	proxy := &httputil.ReverseProxy{Director: director}
	mux := http.NewServeMux()

	var mwares []middlewares.Middleware

	if viper.GetBool("auth-enable") {
		authorizer := auth.New(
			auth.WithAuthorizedDomains(viper.GetString("auth-email-domain")),
		)
		mwares = append(mwares, authorizer.Authorize())
	}

	mux.Handle("/", middlewares.Apply(
		proxy,
		mwares...,
	))

	hostPort := fmt.Sprintf(":%d", viper.GetInt("port"))
	server := &http.Server{Addr: hostPort, Handler: mux}
	lifecycle.ShutdownOnTerm(server)

	go func() {
		internalMux := http.NewServeMux()
		internalMux.Handle("/metrics", promhttp.Handler())
		internalMux.HandleFunc("/live", lifecycle.LivenessHandler)
		internalMux.HandleFunc("/ready", lifecycle.ReadinessHandler)
		zap.L().Info("starting es-proxy metrics server", zap.Int("port", viper.GetInt("metrics-port")))
		http.ListenAndServe(fmt.Sprintf(":%d", viper.GetInt("metrics-port")), internalMux)
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		zap.L().Fatal(err.Error())
	}
}
