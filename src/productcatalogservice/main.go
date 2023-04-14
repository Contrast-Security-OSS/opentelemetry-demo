// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"fmt"
	pb "github.com/opentelemetry/opentelemetry-demo/src/productcatalogservice/genproto/oteldemo"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"google.golang.org/protobuf/encoding/protojson"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

var (
	log               *logrus.Logger
	catalog           []*pb.Product
	resource          *sdkresource.Resource
	initResourcesOnce sync.Once
	actionCounter     syncint64.Counter
)

func init() {
	log = logrus.New()
	catalog = readCatalogFile()
}

func initResource() *sdkresource.Resource {
	initResourcesOnce.Do(func() {
		extraResources, _ := sdkresource.New(
			context.Background(),
			sdkresource.WithOS(),
			sdkresource.WithProcess(),
			sdkresource.WithContainer(),
			sdkresource.WithHost(),
		)
		resource, _ = sdkresource.Merge(
			sdkresource.Default(),
			extraResources,
		)
	})
	return resource
}

func initTracerProvider() *sdktrace.TracerProvider {
	ctx := context.Background()

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		log.Fatalf("OTLP Trace gRPC Creation: %v", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(initResource()),
		sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp
}

func initMeterProvider() *sdkmetric.MeterProvider {
	ctx := context.Background()

	exporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		log.Fatalf("new otlp metric grpc exporter failed: %v", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(initResource()),
	)
	global.SetMeterProvider(mp)
	return mp
}

func main() {
	tp := initTracerProvider()
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Fatalf("Tracer Provider Shutdown: %v", err)
		}
	}()

	mp := initMeterProvider()
	defer func() {
		if err := mp.Shutdown(context.Background()); err != nil {
			log.Fatalf("Error shutting down meter provider: %v", err)
		}
	}()

	err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second))
	if err != nil {
		log.Fatal(err)
	}
	mycounter, err := mp.Meter("productcatalogservice").SyncInt64().Counter(
		"rpc.server.sink",
		instrument.WithUnit("1"),
		instrument.WithDescription("Counts sinktypes seen in a route"),
	)
	if err != nil {
		log.Fatal(err)
	}

	actionCounter = mycounter

	svc := &productCatalog{}
	var port string
	mustMapEnv(&port, "PRODUCT_CATALOG_SERVICE_PORT")
	mustMapEnv(&svc.featureFlagSvcAddr, "FEATURE_FLAG_GRPC_SERVICE_ADDR")

	log.Infof("ProductCatalogService gRPC server started on port: %s", port)

	ln, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("TCP Listen: %v", err)
	}

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
	)

	reflection.Register(srv)

	pb.RegisterProductCatalogServiceServer(srv, svc)
	healthpb.RegisterHealthServer(srv, svc)
	srv.Serve(ln)
}

type productCatalog struct {
	featureFlagSvcAddr string
	pb.UnimplementedProductCatalogServiceServer
}

func readCatalogFile() []*pb.Product {
	catalogJSON, err := ioutil.ReadFile("products.json")
	if err != nil {
		log.Fatalf("Reading Catalog File: %v", err)
	}

	var res pb.ListProductsResponse
	if err := protojson.Unmarshal(catalogJSON, &res); err != nil {
		log.Fatalf("Parsing Catalog JSON: %v", err)
	}

	return res.Products
}

func mustMapEnv(target *string, key string) {
	value, present := os.LookupEnv(key)
	if !present {
		log.Fatalf("Environment Variable Not Set: %q", key)
	}
	*target = value
}

func (p *productCatalog) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (p *productCatalog) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}

func (p *productCatalog) ListProducts(ctx context.Context, req *pb.Empty) (*pb.ListProductsResponse, error) {
	span := trace.SpanFromContext(ctx)

	span.SetAttributes(
		attribute.Int("app.products.count", len(catalog)),
	)
	actionCounter.Add(ctx, 1,
		attribute.String("sinktype", "SinkType.SQL_EXECUTE"),
		attribute.String("rpc.service", "oteldemo.ProductCatalogService"),
		attribute.String("rpc.method", "ListProducts"),
	)
	span.SetAttributes(
		attribute.String("db.system", "mysql"),
		attribute.String("db.name", "astronomystore"),
		attribute.String("db.sql.table", "product.catalog"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("net.peer.name", "mysql1.acme.com"),
	)
	return &pb.ListProductsResponse{Products: catalog}, nil
}

func (p *productCatalog) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.Product, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("app.product.id", req.Id),
	)

	// GetProduct will fail on a specific product when feature flag is enabled
	if p.checkProductFailure(ctx, req.Id) {
		msg := fmt.Sprintf("Error: ProductCatalogService Fail Feature Flag Enabled")
		span.SetStatus(otelcodes.Error, msg)
		span.AddEvent(msg)
		return nil, status.Errorf(codes.Internal, msg)
	}

	actionCounter.Add(ctx, 1,
		attribute.String("sinktype", "SinkType.SQL_EXECUTE"),
		attribute.String("rpc.service", "oteldemo.ProductCatalogService"),
		attribute.String("rpc.method", "GetProduct"),
	)
	span.SetAttributes(
		attribute.String("db.system", "mysql"),
		attribute.String("db.name", "astronomystore"),
		attribute.String("db.sql.table", "product.catalog"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("net.peer.name", "mysql2.acme.com"),
	)
	var found *pb.Product
	for _, product := range catalog {
		if req.Id == product.Id {
			found = product
			break
		}
	}

	if found == nil {
		msg := fmt.Sprintf("Product Not Found: %s", req.Id)
		span.SetStatus(otelcodes.Error, msg)
		span.AddEvent(msg)
		return nil, status.Errorf(codes.NotFound, msg)
	}

	msg := fmt.Sprintf("Product Found - ID: %s, Name: %s", req.Id, found.Name)
	span.AddEvent(msg)
	span.SetAttributes(
		attribute.String("app.product.name", found.Name),
	)
	return found, nil
}

func (p *productCatalog) SearchProducts(ctx context.Context, req *pb.SearchProductsRequest) (*pb.SearchProductsResponse, error) {
	span := trace.SpanFromContext(ctx)

	actionCounter.Add(ctx, 1,
		attribute.String("sinktype", "SinkType.SQL_EXECUTE"),
		attribute.String("rpc.service", "oteldemo.ProductCatalogService"),
		attribute.String("rpc.method", "SearchProducts"),
	)

	span.SetAttributes(
		attribute.String("db.system", "mysql"),
		attribute.String("db.name", "astronomystore"),
		attribute.String("db.sql.table", "product.catalog"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("net.peer.name", "mysql1.oteldemo.org"),
	)
	var result []*pb.Product
	for _, product := range catalog {
		if strings.Contains(strings.ToLower(product.Name), strings.ToLower(req.Query)) ||
			strings.Contains(strings.ToLower(product.Description), strings.ToLower(req.Query)) {
			result = append(result, product)
		}
	}
	span.SetAttributes(
		attribute.Int("app.products_search.count", len(result)),
	)

	return &pb.SearchProductsResponse{Results: result}, nil
}

func (p *productCatalog) checkProductFailure(ctx context.Context, id string) bool {
	span := trace.SpanFromContext(ctx)
	if id != "OLJCESPC7Z" {
		return false
	}

	actionCounter.Add(ctx, 1,
		attribute.String("sinktype", "SinkType.FILE_OPEN_CREATE"),
		attribute.String("rpc.service", "oteldemo.ProductCatalogService"),
		attribute.String("rpc.method", "GetProduct"),
	)

	span.SetAttributes(
		attribute.String("file.open.flags", "R"),
		attribute.String("file.open.path", "product_catalog.json"),
	)
	conn, err := createClient(ctx, p.featureFlagSvcAddr)
	if err != nil {
		span := trace.SpanFromContext(ctx)
		span.AddEvent("error", trace.WithAttributes(attribute.String("message", "Feature Flag Connection Failed")))
		return false
	}
	defer conn.Close()

	flagName := "productCatalogFailure"
	ffResponse, err := pb.NewFeatureFlagServiceClient(conn).GetFlag(ctx, &pb.GetFlagRequest{
		Name: flagName,
	})
	if err != nil {
		span := trace.SpanFromContext(ctx)
		span.AddEvent("error", trace.WithAttributes(attribute.String("message", fmt.Sprintf("GetFlag Failed: %s", flagName))))
		return false
	}

	return ffResponse.GetFlag().Enabled
}

func createClient(ctx context.Context, svcAddr string) (*grpc.ClientConn, error) {
	return grpc.DialContext(ctx, svcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
	)
}
