package tracing

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"net/http"
	"net/url"
)

const defaultComponentName = "net/http"

type mwOptions struct {
	opNameFunc    func(r *http.Request) string
	spanFilter    func(r *http.Request) bool
	spanObserver  func(span opentracing.Span, r *http.Request)
	urlTagFunc    func(u *url.URL) string
	componentName string
}

// MWOption controls the behavior of the Middleware.
type MWOption func(*mwOptions)

// OperationNameFunc returns a MWOption that uses given function f
// to generate operation name for each server-side span.
func OperationNameFunc(f func(r *http.Request) string) MWOption {
	return func(options *mwOptions) {
		options.opNameFunc = f
	}
}

// MWComponentName returns a MWOption that sets the component name
// for the server-side span.
func MWComponentName(componentName string) MWOption {
	return func(options *mwOptions) {
		options.componentName = componentName
	}
}

// MWSpanFilter returns a MWOption that filters requests from creating a span
// for the server-side span.
// Span won't be created if it returns false.
func MWSpanFilter(f func(r *http.Request) bool) MWOption {
	return func(options *mwOptions) {
		options.spanFilter = f
	}
}

// MWSpanObserver returns a MWOption that observe the span
// for the server-side span.
func MWSpanObserver(f func(span opentracing.Span, r *http.Request)) MWOption {
	return func(options *mwOptions) {
		options.spanObserver = f
	}
}

// MWURLTagFunc returns a MWOption that uses given function f
// to set the span's http.url tag. Can be used to change the default
// http.url tag, eg to redact sensitive information.
func MWURLTagFunc(f func(u *url.URL) string) MWOption {
	return func(options *mwOptions) {
		options.urlTagFunc = f
	}
}

// Tracer - use this function as a middleware in your gin router (router.Use(tracer))
func Tracer(tr opentracing.Tracer, options ...MWOption) gin.HandlerFunc {

	// default options
	opts := &mwOptions{
		opNameFunc: func(r *http.Request) string {
			return fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path)
		},
		spanFilter:   func(r *http.Request) bool { return true },
		spanObserver: func(span opentracing.Span, r *http.Request) {},
		urlTagFunc: func(u *url.URL) string {
			return u.String()
		},
	}

	for _, opt := range options {
		opt(opts)
	}

	// set component name, use "net/http" if caller does not specify
	componentName := opts.componentName
	if componentName == "" {
		componentName = defaultComponentName
	}

	handler := func(c *gin.Context) {

		fmt.Println("tracer middleware call")

		if !opts.spanFilter(c.Request) {
			c.Next()
			return
		}

		// retrieve parent span info in the request headers IF EXISTS
		carrier := opentracing.HTTPHeadersCarrier(c.Request.Header)
		ctx, _ := tr.Extract(opentracing.HTTPHeaders, carrier)

		// operation name
		opName := opts.opNameFunc(c.Request)

		// starting a new span for this request
		span := tr.StartSpan(opName, ext.RPCServerOption(ctx))

		// set span tag info
		ext.HTTPMethod.Set(span, c.Request.Method)
		ext.HTTPUrl.Set(span, opts.urlTagFunc(c.Request.URL))
		ext.Component.Set(span, componentName)

		// span observer I dont have a fucking clue what is it for
		opts.spanObserver(span, c.Request)

		// update request info with a new span info - to pass down span info
		c.Request = c.Request.WithContext(
			opentracing.ContextWithSpan(c.Request.Context(), span),
		)

		// custom logs for this specific span
		//span.LogFields(
		//	log.String("event", "http request"),
		//)

		c.Next()

		ext.HTTPStatusCode.Set(span, uint16(c.Writer.Status()))
		span.Finish()

	}

	return handler
}
