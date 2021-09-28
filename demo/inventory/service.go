package main

import (
	"context"
	"github.com/alloykh/tracer-demo/demo/protos/genproto/inventory_service"
	"github.com/alloykh/tracer-demo/log"
	"go.uber.org/zap"

	"github.com/golang/protobuf/ptypes/empty"
)

type Service struct {
	logr *log.Factory
	repo *Repository
}

func NewService(logr *log.Factory, repo *Repository) *Service {
	return &Service{
		logr: logr,
		repo: repo,
	}
}

func (s *Service) AllocateProduct(ctx context.Context, req *inventory_service.AllocProductRequest) (resp *empty.Empty, err error) {

	resp = &empty.Empty{}

	s.logr.Default().Info("GRPC request", zap.Any("info", req))

	err = s.repo.Allocate(ctx, req.Uid, uint64(req.Quantity))

	return
}
