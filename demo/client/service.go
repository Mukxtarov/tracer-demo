package main

import (
	"context"
	"github.com/alloykh/tracer-demo/demo/protos/genproto/client_service"
	"github.com/alloykh/tracer-demo/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type Service struct {
	logr *log.Factory
	data map[string]*Client
}

func NewService(logr *log.Factory) *Service {

	data := map[string]*Client{
		"alloy": {
			UUID: "89ff343a-2a0b-490b-8490-ae7f5728d1e4",
			Name: "alloy",
		},
	}

	return &Service{
		logr: logr,
		data: data,
	}
}

func (s *Service) SearchClient(ctx context.Context, req *client_service.ClientSearchRequest) (resp *client_service.Client, err error) {

	u, ok := s.data[req.Uid]

	if !ok {
		err = status.Error(codes.NotFound, "user not found")
		return
	}

	return &client_service.Client{
		Uid:  u.UUID,
		Name: u.Name,
	}, nil
}
