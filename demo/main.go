package main

import (
	"context"
	"fmt"
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

	opentracing.SetGlobalTracer(tracer)

	httpServer := NewServer("localhost", 8077, logr.Default(), tracer)

	err := httpServer.Run()

	if err != nil {
		logr.Default().Fatal("http server run", zap.Any("err", err.Error()))
	}

	// graceful shutdown
	<-ctx.Done()

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	for _, f := range tearDowns {
		f()
	}

	if err = httpServer.shutdown(ctxShutDown); err != nil {
		logr.Default().Error("http server shutdown", zap.Any("err", err.Error()))
	}

	logr.Default().Info("graceful shutdown")

	os.Exit(0)
}

type server struct {
	router *gin.Engine
	logr   log.Logger
	serv   *http.Server
}

func NewServer(host string, port int, logr log.Logger, tracer opentracing.Tracer) *server {

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
		router: ginRouter,
		logr:   logr,
		serv:   serv,
	}
}

func (s *server) Run() (err error) {

	s.router.GET("/order", orderHandler)

	go func() {
		if err = s.serv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logr.Error("http listen and serve", zap.Any("err", err.Error()))
		}
		s.logr.Info("HTTP SERVER SHUTDOWN", zap.Any("OUTCOME", "successful"))
	}()

	s.logr.Debug("HTTP SERVER RUNNING...", zap.Any("ADDR", s.serv.Addr))

	return
}

func (s *server) shutdown(ctx context.Context) (err error) {
	return s.serv.Shutdown(ctx)
}

func orderHandler(c *gin.Context) {

	// 1. search a client in client service via grpc call (pass span context)

	c.JSON(http.StatusOK, gin.H{"result": "okay"})
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
