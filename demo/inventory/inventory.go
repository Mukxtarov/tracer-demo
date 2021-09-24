package main

import (
	"context"
	"github.com/alloykh/tracer-demo/log"
	"github.com/alloykh/tracer-demo/tracing"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-lib/metrics"
	JProm "github.com/uber/jaeger-lib/metrics/prometheus"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"net"
	"os"
	"os/signal"
)

var tearDowns []func()

var serviceName = "inventory_service"

var defaultGrpcPort = ":7051"

type server struct {
	gRPCServer *grpc.Server
	logr       *log.Factory
	listener   net.Listener
}

func main() {

	ctx := getDefaultContext()

	metricsFactory := JProm.New().Namespace(metrics.NSOptions{Name: serviceName, Tags: nil})

	// logger
	logr := log.NewFactory("zap", zapcore.DebugLevel)

	tracer, tr := tracing.InitJaeger(serviceName, metricsFactory, logr)

	tearDowns = append(tearDowns, tr)

	opentracing.SetGlobalTracer(tracer)


	<-ctx.Done()

	for _, f := range tearDowns {
		f()
	}

	logr.Default().Info("graceful shutdown")

	os.Exit(0)

}


func getDefaultContext() context.Context {

	ctx, cancel := context.WithCancel(context.Background())

	closeChan := make(chan os.Signal, 1)

	signal.Notify(closeChan, os.Interrupt, os.Kill)

	go func() {
		<-closeChan
		cancel()
	}()

	return ctx
}