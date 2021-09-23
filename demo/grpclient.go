package main

import (
	"fmt"
	"github.com/alloykh/tracer-demo/demo/client/protos/genproto/client_service"
	"github.com/alloykh/tracer-demo/log"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"time"

	grpcRetry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	GRPCOpenTracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
)

type Clients struct {
	UserClient client_service.ClientServiceClient
	TearDowns  []func(log *log.Factory)
}

func NewGRPClients() (clients *Clients, err error) {

	clients = &Clients{}

	userClient, tr, err := callToUserClient()
	clients.UserClient = userClient
	clients.TearDowns = append(clients.TearDowns, tr)

	return
}

func callToUserClient() (client_service.ClientServiceClient, func(log *log.Factory), error) {

	connStr := fmt.Sprintf("%v%v", "localhost", ":7081")

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

	conn, err := grpc.Dial(
		connStr,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpcRetry.UnaryClientInterceptor(retryOpts...)),
		grpc.WithUnaryInterceptor(GRPCOpenTracing.UnaryClientInterceptor(tracingOpts...)),
	)

	if err != nil {
		return nil, nil, errors.Wrap(err, "grpc-clients-initLimitServiceClient()")
	}

	tr := func(log *log.Factory) {
		log.Default().Debug("shutting down grpc client") // add name of the client
		if err := conn.Close(); err != nil {
			log.Default().Error("grpc client connection close", zap.Any("err", err.Error()))
		}
	}

	return client_service.NewClientServiceClient(conn), tr, nil

}
