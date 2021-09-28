package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/alloykh/tracer-demo/log"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/sony/gobreaker"
)

var defaultTimeOut = time.Second * 20

type HTTPService struct {
	client *http.Client
	logr   *log.Factory

	cb *gobreaker.CircuitBreaker

	timeOut time.Duration
}

type Option func(client *HTTPService)

func WithProxy(rawUrl string) Option {

	return func(client *HTTPService) {
		tr, ok := client.client.Transport.(*http.Transport)
		if ok {
			proxyUrl, err := url.Parse(rawUrl)
			if err != nil {
				return
			}
			tr.Proxy = http.ProxyURL(proxyUrl)
		}

	}
}

func WithTimeOut(timeOut time.Duration) Option {
	return func(s *HTTPService) {
		if timeOut <= 0 {
			return
		}
		s.client.Timeout = timeOut
	}
}

// NewClient - new http client
func NewClient(logr *log.Factory, opts ...Option) (s *HTTPService) {

	// create a new circuit breaker
	cb := NewCircuitBreaker(logr)

	// create a custom http transport
	tr := http.Transport{
		MaxIdleConns:        150,
		MaxIdleConnsPerHost: 2, // default
		IdleConnTimeout:     time.Second * 100,
		WriteBufferSize:     4 << 10, // default 4 KB
		ReadBufferSize:      4 << 10, // default 4 KB
	}

	// HTTPService - create a new http service
	s = &HTTPService{
		logr: logr,
		cb:   cb,
		client: &http.Client{
			Transport: &nethttp.Transport{RoundTripper: &tr},
			Timeout:   defaultTimeOut,
		},
	}

	// circuit breaker settings
	for _, opt := range opts {
		opt(s)
	}

	return
}

// NewCircuitBreaker - circuit breaker init
func NewCircuitBreaker(logr *log.Factory) *gobreaker.CircuitBreaker {
	settings := gobreaker.Settings{
		Name:        "HTTP",
		MaxRequests: 2,
		Interval:    time.Minute * 5,
		Timeout:     time.Second * 30,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logr.Default().Debug("circuit breaker state change", zap.String("name", name), zap.String("from", from.String()), zap.String("to", to.String()))
		},
	}

	// init circuit breaker
	return gobreaker.NewCircuitBreaker(settings)
}

// Do - execute http request
func (h *HTTPService) Do(ctx context.Context, req *http.Request, resp interface{}) (err error) {

	// if we have open tracing and registered as a global tracer, we start op-span - we inject span context into the http request headers
	if opentracing.IsGlobalTracerRegistered() {
		traceReq, sp := nethttp.TraceRequest(opentracing.GlobalTracer(), req, nethttp.OperationName(fmt.Sprintf("HTTP %s: %s", req.Method, req.URL.Path)))
		req = traceReq
		defer sp.Finish()
	}

	interval := 0
	var rawResp interface{}

	for {

		// execute http.do via circuit breaker
		rawResp, err = h.cb.Execute(func() (interface{}, error) {
			return h.client.Do(req)
		})

		// if no err, break the loop to handle the http response
		if err == nil {
			break
		}

		h.logr.For(ctx).Error("tracing.Do client.Do call", zap.String("err", err.Error()))

		// if circuit breaker is in the open state
		if err == gobreaker.ErrOpenState {
			break
		}

		// wait for some time before retrying
		time.Sleep(5 * (time.Duration(interval) + 1) * time.Second)

		interval++

		h.logr.Default().Debug(fmt.Sprintf("Request to %v trying %v time", req.Host, interval))

	}

	if err != nil {

		h.logr.For(ctx).Error("http do call", zap.String("err", err.Error()))

		return &Error{
			Err:  ErrConnectionFailed,
			Info: err.Error(),
		}
	}

	res := rawResp.(*http.Response)

	defer func() {
		if err := res.Body.Close(); err != nil {
			h.logr.For(ctx).Error("http.Do resp body close", zap.String("err", err.Error()))
		}
	}()

	data, err := io.ReadAll(res.Body)

	if err != nil {
		h.logr.For(ctx).Error("http.Do io.ReadAll", zap.String("err", err.Error()))
		return
	}

	h.logr.For(ctx).Debug("io.ReadAll", zap.String("data", string(data)))

	err = json.Unmarshal(data, &resp)

	if err != nil {
		h.logr.For(ctx).Error("http.Do resp json unmarshal", zap.String("err", err.Error()))
	}

	return
}
