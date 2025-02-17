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
	"net"
	"os"
	"time"
	"strings"
	"errors"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/checkoutservice/genproto"
	money "github.com/GoogleCloudPlatform/microservices-demo/src/checkoutservice/money"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

const (
	listenPort  = "5050"
	usdCurrency = "USD"
)

var log *logrus.Logger

func init() {
	log = logrus.New()
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
}

type checkoutService struct {
	productCatalogSvcAddr string
	cartSvcAddr           string
	currencySvcAddr       string
	shippingSvcAddr       string
	emailSvcAddr          string
	paymentSvcAddr        string
}

func main() {
	if os.Getenv("DISABLE_TRACING") == "" {
		log.Info("Tracing enabled.")
		initOpenTelemetry(log)

	} else {
		log.Info("Tracing disabled.")
	}

	port := listenPort
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}

	svc := new(checkoutService)
	if os.Getenv("SHIPPING_SVC_DISABLED") == "" {
		mustMapEnv(&svc.shippingSvcAddr, "SHIPPING_SERVICE_ADDR")
	}
	mustMapEnv(&svc.productCatalogSvcAddr, "PRODUCT_CATALOG_SERVICE_ADDR")
	mustMapEnv(&svc.cartSvcAddr, "CART_SERVICE_ADDR")
	mustMapEnv(&svc.currencySvcAddr, "CURRENCY_SERVICE_ADDR")
	if os.Getenv("EMAIL_SVC_DISABLED") == "" {
		mustMapEnv(&svc.emailSvcAddr, "EMAIL_SERVICE_ADDR")
	}
	if os.Getenv("PAYMENT_SVC_DISABLED") == "" {
		mustMapEnv(&svc.paymentSvcAddr, "PAYMENT_SERVICE_ADDR")
	}

	log.Infof("service config: %+v", svc)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}

	var srv *grpc.Server

	if os.Getenv("DISABLE_TRACING") == "" {
		srv = grpc.NewServer(
			grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
			grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
		)
	} else {
		srv = grpc.NewServer()
	}


	pb.RegisterCheckoutServiceServer(srv, svc)
	healthpb.RegisterHealthServer(srv, svc)
	log.Infof("starting to listen on tcp: %q", lis.Addr().String())
	err = srv.Serve(lis)
	log.Fatal(err)
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

	log.Info("created jaeger exporter to agent at " + svcAddr)

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exporter, tracesdk.WithMaxExportBatchSize(95)),
		// see https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace#ParentBased
		tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.AlwaysSample())),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("checkoutservice"),
		)),
	)
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

func (cs *checkoutService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (cs *checkoutService) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}

func (cs *checkoutService) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
	log.Infof("[PlaceOrder] user_id=%q user_currency=%q", req.UserId, req.UserCurrency)

	orderID, err := uuid.NewUUID()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate order uuid")
	}

	prep, err := cs.prepareOrderItemsAndShippingQuoteFromCart(ctx, req.UserId, req.UserCurrency, req.Address)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	total := pb.Money{CurrencyCode: req.UserCurrency,
		Units: 0,
		Nanos: 0}
	total = money.Must(money.Sum(total, *prep.shippingCostLocalized))
	for _, it := range prep.orderItems {
		multPrice := money.MultiplySlow(*it.Cost, uint32(it.GetItem().GetQuantity()))
		total = money.Must(money.Sum(total, multPrice))
	}

	txID, err := cs.chargeCard(ctx, &total, req.CreditCard)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to charge card: %+v", err)
	}
	log.Infof("payment went through (transaction_id: %s)", txID)

	shippingTrackingID, err := cs.shipOrder(ctx, req.Address, prep.cartItems)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "shipping error: %+v", err)
	}

	_ = cs.emptyUserCart(ctx, req.UserId)

	orderResult := &pb.OrderResult{
		OrderId:            orderID.String(),
		ShippingTrackingId: shippingTrackingID,
		ShippingCost:       prep.shippingCostLocalized,
		ShippingAddress:    req.Address,
		Items:              prep.orderItems,
	}

	if err := cs.sendOrderConfirmation(ctx, req.Email, orderResult); err != nil {
		log.Warnf("failed to send order confirmation to %q: %+v", req.Email, err)
	} else {
		log.Infof("order confirmation email sent to %q", req.Email)
	}
	resp := &pb.PlaceOrderResponse{Order: orderResult}
	return resp, nil
}

type orderPrep struct {
	orderItems            []*pb.OrderItem
	cartItems             []*pb.CartItem
	shippingCostLocalized *pb.Money
}

func (cs *checkoutService) prepareOrderItemsAndShippingQuoteFromCart(ctx context.Context, userID, userCurrency string, address *pb.Address) (orderPrep, error) {
	var out orderPrep
	cartItems, err := cs.getUserCart(ctx, userID)
	if err != nil {
		return out, fmt.Errorf("cart failure: %+v", err)
	}
	orderItems, err := cs.prepOrderItems(ctx, cartItems, userCurrency)
	if err != nil {
		return out, fmt.Errorf("failed to prepare order: %+v", err)
	}
	shippingUSD, err := cs.quoteShipping(ctx, address, cartItems)
	if err != nil {
		return out, fmt.Errorf("shipping quote failure: %+v", err)
	}
	shippingPrice, err := cs.convertCurrency(ctx, shippingUSD, userCurrency)
	if err != nil {
		return out, fmt.Errorf("failed to convert shipping cost to currency: %+v", err)
	}

	out.shippingCostLocalized = shippingPrice
	out.cartItems = cartItems
	out.orderItems = orderItems
	return out, nil
}

func (cs *checkoutService) quoteShipping(ctx context.Context, address *pb.Address, items []*pb.CartItem) (*pb.Money, error) {

	if os.Getenv("SHIPPING_SVC_DISABLED") != "" {
		log.Info("Shipping service disabled. Mocking call, always 5.00 USD shipping quote.")

		return &pb.Money{
				CurrencyCode: "USD",
				Units:        int64(5),
				Nanos:        int32(0)},
			error(nil)
	}

	var conn *grpc.ClientConn
	var err error
	if os.Getenv("DISABLE_TRACING") == "" {
		conn, err = grpc.DialContext(ctx, cs.shippingSvcAddr,
			grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()), grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	} else {
		conn, err = grpc.DialContext(ctx, cs.shippingSvcAddr,
			grpc.WithInsecure())
	}

	if err != nil {
		return nil, fmt.Errorf("could not connect shipping service: %+v", err)
	}
	defer conn.Close()

	shippingQuote, err := pb.NewShippingServiceClient(conn).
		GetQuote(ctx, &pb.GetQuoteRequest{
			Address: address,
			Items:   items})
	if err != nil {
		return nil, fmt.Errorf("failed to get shipping quote: %+v", err)
	}
	return shippingQuote.GetCostUsd(), nil
}

func (cs *checkoutService) getUserCart(ctx context.Context, userID string) ([]*pb.CartItem, error) {
	var conn *grpc.ClientConn
	var err error
	if os.Getenv("DISABLE_TRACING") == "" {
		conn, err = grpc.DialContext(ctx, cs.cartSvcAddr, grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()), grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	} else {
		conn, err = grpc.DialContext(ctx, cs.cartSvcAddr, grpc.WithInsecure())
	}

	if err != nil {
		return nil, fmt.Errorf("could not connect cart service: %+v", err)
	}
	defer conn.Close()

	cart, err := pb.NewCartServiceClient(conn).GetCart(ctx, &pb.GetCartRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("failed to get user cart during checkout: %+v", err)
	}
	return cart.GetItems(), nil
}

func (cs *checkoutService) emptyUserCart(ctx context.Context, userID string) error {
	var conn *grpc.ClientConn
	var err error
	if os.Getenv("DISABLE_TRACING") == "" {
		conn, err = grpc.DialContext(ctx, cs.cartSvcAddr, grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()), grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	} else {
		conn, err = grpc.DialContext(ctx, cs.cartSvcAddr, grpc.WithInsecure())
	}

	if err != nil {
		return fmt.Errorf("could not connect cart service: %+v", err)
	}
	defer conn.Close()

	if _, err = pb.NewCartServiceClient(conn).EmptyCart(ctx, &pb.EmptyCartRequest{UserId: userID}); err != nil {
		return fmt.Errorf("failed to empty user cart during checkout: %+v", err)
	}
	return nil
}

func (cs *checkoutService) prepOrderItems(ctx context.Context, items []*pb.CartItem, userCurrency string) ([]*pb.OrderItem, error) {
	out := make([]*pb.OrderItem, len(items))

	var conn *grpc.ClientConn
	var err error
	if os.Getenv("DISABLE_TRACING") == "" {
		conn, err = grpc.DialContext(ctx, cs.productCatalogSvcAddr, grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()), grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	} else {
		conn, err = grpc.DialContext(ctx, cs.productCatalogSvcAddr, grpc.WithInsecure())
	}

	if err != nil {
		return nil, fmt.Errorf("could not connect product catalog service: %+v", err)
	}
	defer conn.Close()
	cl := pb.NewProductCatalogServiceClient(conn)

	for i, item := range items {
		product, err := cl.GetProduct(ctx, &pb.GetProductRequest{Id: item.GetProductId()})
		if err != nil {
			return nil, fmt.Errorf("failed to get product #%q", item.GetProductId())
		}
		price, err := cs.convertCurrency(ctx, product.GetPriceUsd(), userCurrency)
		if err != nil {
			return nil, fmt.Errorf("failed to convert price of %q to %s", item.GetProductId(), userCurrency)
		}
		out[i] = &pb.OrderItem{
			Item: item,
			Cost: price}
	}
	return out, nil
}

func (cs *checkoutService) convertCurrency(ctx context.Context, from *pb.Money, toCurrency string) (*pb.Money, error) {
	var conn *grpc.ClientConn
	var err error
	if os.Getenv("DISABLE_TRACING") == "" {
		conn, err = grpc.DialContext(ctx, cs.currencySvcAddr, grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()), grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	} else {
		conn, err = grpc.DialContext(ctx, cs.currencySvcAddr, grpc.WithInsecure())
	}


	if err != nil {
		return nil, fmt.Errorf("could not connect currency service: %+v", err)
	}
	defer conn.Close()
	result, err := pb.NewCurrencyServiceClient(conn).Convert(ctx, &pb.CurrencyConversionRequest{
		From:   from,
		ToCode: toCurrency})
	if err != nil {
		return nil, fmt.Errorf("failed to convert currency: %+v", err)
	}
	return result, err
}

func (cs *checkoutService) chargeCard(ctx context.Context, amount *pb.Money, paymentInfo *pb.CreditCardInfo) (string, error) {
	if os.Getenv("PAYMENT_SVC_DISABLED") != "" {
		log.Info("Payment service disabled. Mocking call, always return 'Mock_Transaction_ID'")
		return "Mock_Transaction_ID", nil
	}
	var conn *grpc.ClientConn
	var err error
	if os.Getenv("DISABLE_TRACING") == "" {
		conn, err = grpc.DialContext(ctx, cs.paymentSvcAddr, grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()), grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	} else {
		conn, err = grpc.DialContext(ctx, cs.paymentSvcAddr, grpc.WithInsecure())
	}

	if err != nil {
		return "", fmt.Errorf("failed to connect payment service: %+v", err)
	}
	defer conn.Close()

	paymentResp, err := pb.NewPaymentServiceClient(conn).Charge(ctx, &pb.ChargeRequest{
		Amount:     amount,
		CreditCard: paymentInfo})
	if err != nil {
		return "", fmt.Errorf("could not charge the card: %+v", err)
	}
	return paymentResp.GetTransactionId(), nil
}

func (cs *checkoutService) sendOrderConfirmation(ctx context.Context, email string, order *pb.OrderResult) error {
	if os.Getenv("EMAIL_SVC_DISABLED") != "" {
		log.Info("Email Service disabled. Skipping call.")
		return nil
	}
	var conn *grpc.ClientConn
	var err error
	if os.Getenv("DISABLE_TRACING") == "" {
		conn, err = grpc.DialContext(ctx, cs.emailSvcAddr, grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()), grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	} else {
		conn, err = grpc.DialContext(ctx, cs.emailSvcAddr, grpc.WithInsecure())
	}

	if err != nil {
		return fmt.Errorf("failed to connect email service: %+v", err)
	}
	defer conn.Close()
	_, err = pb.NewEmailServiceClient(conn).SendOrderConfirmation(ctx, &pb.SendOrderConfirmationRequest{
		Email: email,
		Order: order})
	return err
}

func (cs *checkoutService) shipOrder(ctx context.Context, address *pb.Address, items []*pb.CartItem) (string, error) {

	if os.Getenv("SHIPPING_SVC_DISABLED") != "" {
		log.Info("Shipping service disabled. Mocking call, always return 'Mock_Tracking_ID'")
		return "Mock_Tracking_ID", nil
	}

	var conn *grpc.ClientConn
	var err error
	if os.Getenv("DISABLE_TRACING") == "" {
		conn, err = grpc.DialContext(ctx, cs.shippingSvcAddr, grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()), grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	} else {
		conn, err = grpc.DialContext(ctx, cs.shippingSvcAddr, grpc.WithInsecure())
	}

	if err != nil {
		return "", fmt.Errorf("failed to connect email service: %+v", err)
	}
	defer conn.Close()
	resp, err := pb.NewShippingServiceClient(conn).ShipOrder(ctx, &pb.ShipOrderRequest{
		Address: address,
		Items:   items})
	if err != nil {
		return "", fmt.Errorf("shipment failed: %+v", err)
	}
	return resp.GetTrackingId(), nil
}

// TODO: Dial and create client once, reuse.
