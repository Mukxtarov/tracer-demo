package main

import (
	"context"
	"fmt"
	"github.com/alloykh/tracer-demo/demo/protos/genproto/client_service"
	"github.com/alloykh/tracer-demo/log"
	"github.com/alloykh/tracer-demo/tracing"

	GRPCMiddleware "github.com/grpc-ecosystem/go-grpc-middleware"        // grpc interceptors https://github.com/grpc-ecosystem/go-grpc-middleware
	GRPCRecovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery" // grpc interceptors https://github.com/grpc-ecosystem/go-grpc-middleware
	GRPCCtxTags "github.com/grpc-ecosystem/go-grpc-middleware/tags"      // grpc interceptors https://github.com/grpc-ecosystem/go-grpc-middleware
	GRPCOpenTracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	//GRPCZap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-lib/metrics"
	JProm "github.com/uber/jaeger-lib/metrics/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
	"os"
	"os/signal"
)

var tearDowns []func()

var serviceName = "client_service"

var defaultGrpcPort = ":7050"

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

	// setup grpc server
	serv, tr := newGrpcServer(logr)

	tearDowns = append(tearDowns, tr)

	if err := serv.setListener(); err != nil {
		logr.Default().Fatal("server set listener", zap.Any("err", err.Error()))
		return
	}

	go func() {

		if err := serv.run(); err != nil {
			logr.Default().Info(fmt.Sprintf("error while running grpc server: %v", err.Error()))
		}

	}()

	<-ctx.Done()

	//_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer func() {
	//	cancel()
	//}()

	for _, f := range tearDowns {
		f()
	}

	logr.Default().Info("graceful shutdown")

	os.Exit(0)

}

func newGrpcServer(logr *log.Factory) (*server, func()) {

	s := grpc.NewServer(
		GRPCMiddleware.WithUnaryServerChain(
			GRPCRecovery.UnaryServerInterceptor(),
			GRPCCtxTags.UnaryServerInterceptor(),
			//GRPCZap.UnaryServerInterceptor(logr.Default()),
			GRPCOpenTracing.UnaryServerInterceptor(), // we can later add those interceptors -
			// prometheus.UnaryServerInterceptor,  // - for authentication and monitoring purposes
			// auth.UnaryServerInterceptor(myAuthFunction),
		),
	)

	teardown := func() {
		s.GracefulStop()
		logr.Default().Info("grpc server has been shut down")
	}

	return &server{
		gRPCServer: s,
		logr:       logr,
	}, teardown

}

func (s *server) setListener() error {

	listener, errOnListen := net.Listen("tcp", defaultGrpcPort)

	if errOnListen != nil {

		s.logr.Default().Error(errOnListen.Error())

		return errOnListen
	}

	s.listener = listener

	return nil
}

func (s *server) run() (err error) {

	service := NewService(s.logr)

	client_service.RegisterClientServiceServer(s.gRPCServer, service)

	reflection.Register(s.gRPCServer)

	s.logr.Default().Info(fmt.Sprintf("GRPC Server started at port %v", defaultGrpcPort))

	err = s.gRPCServer.Serve(s.listener)

	if err != nil {

		s.logr.Default().Error("grpc server serve", zap.Any("err", err.Error()))

	}

	return
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
