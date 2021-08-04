// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
	"strings"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

const (
	port            = "8080"
	defaultCurrency = "USD"
	cookieMaxAge    = 60 * 60 * 48

	cookiePrefix    = "shop_"
	cookieSessionID = cookiePrefix + "session-id"
	cookieCurrency  = cookiePrefix + "currency"
)

var (
	whitelistedCurrencies = map[string]bool{
		"USD": true,
		"EUR": true,
		"CAD": true,
		"JPY": true,
		"GBP": true,
		"TRY": true}
)

type ctxKeySessionID struct{}

type frontendServer struct {
	productCatalogSvcAddr string
	productCatalogSvcConn *grpc.ClientConn

	currencySvcAddr string
	currencySvcConn *grpc.ClientConn

	cartSvcAddr string
	cartSvcConn *grpc.ClientConn

	recommendationSvcAddr string
	recommendationSvcConn *grpc.ClientConn

	checkoutSvcAddr string
	checkoutSvcConn *grpc.ClientConn

	shippingSvcAddr string
	shippingSvcConn *grpc.ClientConn

	adSvcAddr string
	adSvcConn *grpc.ClientConn
}

func main() {
	ctx := context.Background()
	log := logrus.New()
	log.Level = logrus.DebugLevel
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout

	if os.Getenv("DISABLE_TRACING") == "" {
		log.Info("Tracing enabled.")
		go initOpenTelemetry(log)

	} else {
		log.Info("Tracing disabled.")
	}

	srvPort := port
	if os.Getenv("PORT") != "" {
		srvPort = os.Getenv("PORT")
	}
	addr := os.Getenv("LISTEN_ADDR")
	svc := new(frontendServer)
	mustMapEnv(&svc.productCatalogSvcAddr, "PRODUCT_CATALOG_SERVICE_ADDR")
	mustMapEnv(&svc.currencySvcAddr, "CURRENCY_SERVICE_ADDR")
	mustMapEnv(&svc.cartSvcAddr, "CART_SERVICE_ADDR")
	if os.Getenv("RECOMMENDATION_SVC_DISABLED") == "" {
		mustMapEnv(&svc.recommendationSvcAddr, "RECOMMENDATION_SERVICE_ADDR")
	}
	mustMapEnv(&svc.checkoutSvcAddr, "CHECKOUT_SERVICE_ADDR")
	if os.Getenv("SHIPPING_SVC_DISABLED") == "" {
		mustMapEnv(&svc.shippingSvcAddr, "SHIPPING_SERVICE_ADDR")
	}
	mustMapEnv(&svc.adSvcAddr, "AD_SERVICE_ADDR")

	mustConnGRPC(ctx, &svc.currencySvcConn, svc.currencySvcAddr)
	mustConnGRPC(ctx, &svc.productCatalogSvcConn, svc.productCatalogSvcAddr)
	mustConnGRPC(ctx, &svc.cartSvcConn, svc.cartSvcAddr)
	if os.Getenv("RECOMMENDATION_SVC_DISABLED") == "" {
		mustConnGRPC(ctx, &svc.recommendationSvcConn, svc.recommendationSvcAddr)
	}
	if os.Getenv("SHIPPING_SVC_DISABLED") == "" {
		mustConnGRPC(ctx, &svc.shippingSvcConn, svc.shippingSvcAddr)
	}
	mustConnGRPC(ctx, &svc.checkoutSvcConn, svc.checkoutSvcAddr)
	mustConnGRPC(ctx, &svc.adSvcConn, svc.adSvcAddr)

	r := mux.NewRouter()
	r.HandleFunc("/", svc.homeHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/product/{id}", svc.productHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/cart", svc.viewCartHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/cart", svc.addToCartHandler).Methods(http.MethodPost)
	r.HandleFunc("/cart/empty", svc.emptyCartHandler).Methods(http.MethodPost)
	r.HandleFunc("/setCurrency", svc.setCurrencyHandler).Methods(http.MethodPost)
	r.HandleFunc("/logout", svc.logoutHandler).Methods(http.MethodGet)
	r.HandleFunc("/cart/checkout", svc.placeOrderHandler).Methods(http.MethodPost)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	r.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "User-agent: *\nDisallow: /") })
	r.HandleFunc("/_healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") })

	var handler http.Handler = r
	handler = &logHandler{log: log, next: handler} // add logging
	handler = ensureSessionID(handler)             // add session ID

	log.Infof("starting server on " + addr + ":" + srvPort)
	log.Fatal(http.ListenAndServe(addr+":"+srvPort, handler))
}

// for reference, see also:
// https://github.com/open-telemetry/opentelemetry-go/blob/main/example/jaeger/main.go
func createTracerProvider(log logrus.FieldLogger) (*tracesdk.TracerProvider, error) {

	// Create the Jaeger exporter

	svcAddr := os.Getenv("JAEGER_SERVICE_ADDR")
	if svcAddr == "" {
		return nil, errors.New("missing JAEGER_SERVICE_ADDR, can't initialize Jaeger exporter")
	}

	splitJaegerAddr := strings.Split(svcAddr, ":")
	jaegerAgentHost := splitJaegerAddr[0]
	jaegerAgentPort := splitJaegerAddr[1]

	exporter, err := jaeger.New(jaeger.WithAgentEndpoint(jaeger.WithAgentHost(jaegerAgentHost), jaeger.WithAgentPort(jaegerAgentPort)));
	if err != nil {
		return nil, err
	}

	log.Info("created jaeger exporter to collector at " + svcAddr)

	var tp *tracesdk.TracerProvider

	if os.Getenv("TRACES_SAMPLING_FRACTION") == "" {
		log.Info("No sampling applied, choosing ParentBased(AlwaysSample)")
		tp = tracesdk.NewTracerProvider(
			tracesdk.WithBatcher(exporter),
			// see https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace#ParentBased
			tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.AlwaysSample())),
			tracesdk.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String("frontend"),
			)),
		)
	} else {
		fraction, err := strconv.ParseFloat(os.Getenv("TRACES_SAMPLING_FRACTION"), 64)
		if err != nil {
			panic(err)
		}
		log.Info(fmt.Sprintf("Applying sampling with fraction %v", fraction))
		tp = tracesdk.NewTracerProvider(
			tracesdk.WithBatcher(exporter),
			// see https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace#TraceIDRatioBased
			tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(fraction))),
			tracesdk.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String("frontend"),
			)),
		)
	}

	

	return tp, nil
}

func initOpenTelemetry(log logrus.FieldLogger) {



	tp, err := createTracerProvider(log)
	if err != nil {
		log.Fatal(err)
	}

	otel.SetTracerProvider(tp)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

}

func mustMapEnv(target *string, envKey string) {
	v := os.Getenv(envKey)
	if v == "" {
		panic(fmt.Sprintf("environment variable %q not set", envKey))
	}
	*target = v
}

func mustConnGRPC(ctx context.Context, conn **grpc.ClientConn, addr string) {
	var err error

	if os.Getenv("DISABLE_TRACING") == "" {
		*conn, err = grpc.DialContext(ctx, addr,
			grpc.WithInsecure(),
			grpc.WithTimeout(time.Second*3),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
			grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
		)
	} else {
		*conn, err = grpc.DialContext(ctx, addr,
			grpc.WithInsecure(),
			grpc.WithTimeout(time.Second*3),
		)
	}





	if err != nil {
		panic(errors.Wrapf(err, "grpc: failed to connect %s", addr))
	}
}
