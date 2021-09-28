package main

import (
	"context"
	"fmt"
	"github.com/alloykh/tracer-demo/demo/helpers"
	"github.com/alloykh/tracer-demo/demo/protos/genproto/inventory_service"
	"github.com/alloykh/tracer-demo/log"
	"github.com/alloykh/tracer-demo/tracing"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/uber/jaeger-lib/metrics"
	JProm "github.com/uber/jaeger-lib/metrics/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"os"
	"os/signal"
	"time"
)

var tearDowns []func()

var serviceName = "order_service"
var orderServicePort = 8078

func main() {

	ctx := getDefaultContext()

	metricsFactory := JProm.New().Namespace(metrics.NSOptions{Name: serviceName, Tags: nil})

	// logger
	logr := log.NewFactory("zap", zapcore.DebugLevel)

	tracer, tr := tracing.InitJaeger(serviceName, metricsFactory, logr)

	tearDowns = append(tearDowns, tr)

	opentracing.SetGlobalTracer(tracer)

	grpclients, err := NewGRPClients()

	if err != nil {
		logr.Default().Fatal("grpc clients init", zap.Any("err", err.Error()))
	}

	httpServer := NewServer("localhost", orderServicePort, logr, tracer, grpclients)

	err = httpServer.Run()

	if err != nil {
		logr.Default().Fatal("http server run", zap.Any("err", err.Error()))
	}

	// interruption signal - graceful shutdown
	<-ctx.Done()

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	for _, f := range tearDowns {
		f()
	}

	for _, f := range grpclients.TearDowns {
		f(logr)
	}

	if err = httpServer.shutdown(ctxShutDown); err != nil {
		logr.Default().Error("http server shutdown", zap.Any("err", err.Error()))
	}

	logr.Default().Info("graceful shutdown")

	os.Exit(0)
}

type server struct {
	router *gin.Engine
	logr   *log.Factory
	serv   *http.Server

	grpclients *Clients
}

func NewServer(host string, port int, logr *log.Factory, tracer opentracing.Tracer, grpclients *Clients) *server {

	ginRouter := gin.New()

	ginRouter.Use(gin.Recovery())
	ginRouter.Use(tracing.Tracer(tracer))

	serv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Handler:      ginRouter,
		ReadTimeout:  time.Second * 7,
		WriteTimeout: time.Second * 5,
	}

	return &server{
		router:     ginRouter,
		logr:       logr,
		serv:       serv,
		grpclients: grpclients,
	}
}

func (s *server) Run() (err error) {

	s.router.GET("/order", s.orderHandler)

	go func() {
		if err = s.serv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logr.Default().Error("http listen and serve", zap.Any("err", err.Error()))
		}
		s.logr.Default().Info("HTTP SERVER SHUTDOWN", zap.Any("OUTCOME", "successful"))
	}()

	s.logr.Default().Debug("HTTP SERVER RUNNING...", zap.Any("ADDR", s.serv.Addr))

	return
}

func (s *server) shutdown(ctx context.Context) (err error) {
	return s.serv.Shutdown(ctx)
}

type orderResponse struct {
	OrderUID string `json:"order_uid"`
}

func (s *server) orderHandler(c *gin.Context) {

	ctx := c.Request.Context()

	type model struct {
		ClientUUID  string `json:"client_uuid" binding:"required"`
		ProductUUID string `json:"product_uuid" binding:"required"`
		Quantity    uint32 `json:"quantity" binding:"required"`
	}

	m := &model{}

	if err := c.ShouldBindJSON(m); err != nil {
		s.logr.For(ctx).Error(fmt.Sprintf("error while json body binding: %v\n", err.Error()))
		helpers.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err := s.grpclients.InventoryClient.AllocateProduct(ctx, &inventory_service.AllocProductRequest{Uid: m.ProductUUID, Quantity: m.Quantity})

	if err != nil {

		sts := status.Convert(err)

		switch sts.Code() {
		case codes.InvalidArgument:
			helpers.RespondError(c, http.StatusBadRequest, sts.Message())
		case codes.NotFound:
			helpers.RespondError(c, http.StatusNotFound, sts.Message())
		default:
			helpers.RespondError(c, http.StatusInternalServerError, err.Error())
		}

		return
	}

	s.logr.For(ctx).Debug("to order request", zap.Any("input", m))

	helpers.RespondOK(c, &orderResponse{
		OrderUID: "uid-order",
	})
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
