package main

import (
	"context"
	"github.com/alloykh/tracer-demo/log"
	"github.com/alloykh/tracer-demo/tracing"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strconv"
	"sync"
)

type Product struct {
	ID       string
	Name     string
	Quantity uint64
}

type Repository struct {
	logr *log.Factory
	data map[string]*Product
	sync.RWMutex
}

func NewRepo(logr *log.Factory) *Repository {
	data := map[string]*Product{
		"uid-iphone": {
			ID:       "uid-iphone",
			Name:     "Iphone X",
			Quantity: 100,
		},

		"uid-nokia": {
			ID:       "uid-nokia",
			Name:     "Nokia 6230",
			Quantity: 200,
		},
	}

	return &Repository{
		data: data,
		logr: logr,
	}
}

func (r *Repository) Get(ctx context.Context, id string) (p *Product, err error) {

	span, ctx := tracing.NewDBSpanFromContext(ctx)
	defer span.Finish()

	p, ok := r.data[id]

	if !ok {
		err = status.Error(codes.NotFound, "product id was not found")
		return
	}

	return
}

func (r *Repository) Allocate(ctx context.Context, id string, quantity uint64) (err error) {

	span, ctx := tracing.NewDBSpanFromContext(ctx, id, quantity)
	defer span.Finish()
	tracing.WrapWithTags(span, "psql", "update set quantity = $1 where uid = $2") // or some update operation

	p, ok := r.data[id]

	if !ok {
		r.logr.For(ctx).Error("get by uid", zap.String("id", id))
		err = status.Error(codes.NotFound, "product was not found")
		return
	}

	r.Lock()
	defer r.Unlock()

	r.logr.For(ctx).Debug("allocate", zap.String("id", id), zap.String("requested", strconv.FormatUint(quantity, 10)), zap.String("available", strconv.FormatUint(p.Quantity, 10)))

	if p.Quantity < quantity {
		r.logr.For(ctx).Error("product quantity low", zap.String("quantity", strconv.FormatUint(p.Quantity, 10)))
		err = status.Error(codes.InvalidArgument, "quantity is out of range")
		return
	}

	p.Quantity -= quantity

	return
}
