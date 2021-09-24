package main

import (
	"context"
	"fmt"
	"github.com/alloykh/tracer-demo/demo/protos/genproto/client_service"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/pkg/errors"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/alloykh/tracer-demo/log"
	"github.com/alloykh/tracer-demo/tracing"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-lib/metrics"
	jprom "github.com/uber/jaeger-lib/metrics/prometheus"
)

// Front end - service

var serviceName = "frontend"

var tearDowns []func()

func main() {

	ctx := getDefaultContext()

	metricsFactory := jprom.New().Namespace(metrics.NSOptions{Name: serviceName, Tags: nil})

	// logger
	logr := log.NewFactory("zap", zapcore.DebugLevel)

	//	initialize jaeger tracer
	tracer, tr := tracing.InitJaeger(serviceName, metricsFactory, logr)
	tearDowns = append(tearDowns, tr)

	// Set tracer as global
	opentracing.SetGlobalTracer(tracer)

	// init grpc clients
	grpclients, err := NewGRPClients()
	if err != nil {
		logr.Default().Fatal("grpc clients init", zap.Any("err", err.Error()))
	}

	httpServer := NewServer("localhost", 8077, logr, tracer, grpclients)

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
	router     *gin.Engine
	logr       *log.Factory
	serv       *http.Server
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

func (s *server) orderHandler(c *gin.Context) {

	ctx := c.Request.Context()

	// 1. search a client in client service via grpc call (pass span context)
	user, err := s.grpclients.UserClient.SearchClient(ctx, &client_service.ClientSearchRequest{Uid: "uuid"})

	if err != nil {
		s.logr.For(ctx).Error("search client call", zap.String("err", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}


	// 2. Call order service via http



	// custom spanning
	span, _ := opentracing.StartSpanFromContext(c.Request.Context(), "handler response c.Json")
	defer span.Finish()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
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
