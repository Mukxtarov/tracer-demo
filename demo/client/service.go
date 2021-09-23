package main

import (
	"context"
	"github.com/alloykh/tracer-demo/demo/client/protos/genproto/client_service"
	"github.com/alloykh/tracer-demo/log"
	"go.uber.org/zap"
)

type Service struct {
	logr *log.Factory
}

func NewService(logr *log.Factory) *Service {
	return &Service{logr: logr}
}

func (s *Service) SearchClient(ctx context.Context, req *client_service.ClientSearchRequest) (resp *client_service.Client, err error) {

	s.logr.Default().Info("GRPC request", zap.Any("info", req))

	return &client_service.Client{
		Uid:  "89ff343a-2a0b-490b-8490-ae7f5728d1e4",
		Name: "Alloy",
	}, nil
}
