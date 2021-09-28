package main

import (
	"fmt"
	"github.com/alloykh/tracer-demo/demo/protos/genproto/inventory_service"
	"github.com/alloykh/tracer-demo/log"
	grpcRetry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	GRPCOpenTracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"time"
)

type Clients struct {
	InventoryClient inventory_service.InventoryServiceClient
	TearDowns       []func(log *log.Factory)
}


func NewGRPClients() (clients *Clients, err error) {

	clients = &Clients{}

	retryOpts := []grpcRetry.CallOption{
		grpcRetry.WithBackoff(grpcRetry.BackoffLinear(100 * time.Millisecond)),
		grpcRetry.WithCodes(codes.NotFound, codes.Aborted),
	}

	tracingOpts := []GRPCOpenTracing.Option{
		GRPCOpenTracing.WithTracer(opentracing.GlobalTracer()), // setting tracer
		//GRPCOpenTracing.WithOpName(func(method string) string {    // changing operation name
		//	return "hell no"
		//}),
	}

	interceptors := grpc.WithChainUnaryInterceptor(grpcRetry.UnaryClientInterceptor(retryOpts...), GRPCOpenTracing.UnaryClientInterceptor(tracingOpts...))

	inventoryClient, tr, err := callToInventoryClient(grpc.WithInsecure(), interceptors)

	clients.InventoryClient = inventoryClient

	clients.TearDowns = append(clients.TearDowns, tr)

	return
}



func callToInventoryClient(opt ...grpc.DialOption) (inventory_service.InventoryServiceClient, func(log *log.Factory), error) {

	connStr := fmt.Sprintf("%v%v", "localhost", ":7051")

	conn, err := grpc.Dial(
		connStr,
		opt...,
	)

	if err != nil {
		return nil, nil, errors.Wrap(err, "grpc-clients-callToInventoryClient()")
	}

	tr := func(log *log.Factory) {
		log.Default().Debug("shutting down grpc client") // add name of the client
		if err := conn.Close(); err != nil {
			log.Default().Error("grpc client connection close", zap.Any("err", err.Error()))
		}
	}

	return inventory_service.NewInventoryServiceClient(conn), tr, nil

}
