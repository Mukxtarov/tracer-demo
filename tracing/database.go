package tracing

import (
	"context"
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// NewDBSpanFromContext - returns new span for the database operations
// if ctx does not contain span-context, it will not trace the given operation by returning the noop span from noop tracer.
func NewDBSpanFromContext(ctx context.Context, params ...interface{}) (opentracing.Span, context.Context) {

	if span := opentracing.SpanFromContext(ctx); span != nil {
		// if exists start a new span with a new operation name

		span := opentracing.StartSpan("Product Get", opentracing.ChildOf(span.Context()))

		for i, p := range params {
			s := fmt.Sprintf("param.#%d", i)
			span.SetTag(s, p)
		}

		ctx = opentracing.ContextWithSpan(ctx, span)
		return span, ctx
	}

	return opentracing.NoopTracer{}.StartSpan("noop span"), ctx
}

func WrapWithTags(span opentracing.Span, dbType, query string) {
	ext.SpanKindRPCClient.Set(span)
	ext.PeerService.Set(span, dbType) // can be any database call
	span.SetTag("query", query)
}
