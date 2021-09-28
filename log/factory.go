package log

import (
	"context"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Factory struct {
	logger Logger
	tr     func()
}

func NewFactory(name string, level zapcore.Level) *Factory {
	// for now, zap log should be enough
	logger, tr := NewZapLogger(level)
	return &Factory{
		logger: logger,
		tr:     tr,
	}
}

func (f *Factory) Default() Logger {
	return f.logger
}

// For returns a context-aware Logger. If the context
// contains an OpenTracing span, all logging calls are also
// echo-ed into the span.
func (f Factory) For(ctx context.Context) Logger {

	if span := opentracing.SpanFromContext(ctx); span != nil {

		logger := spanLogger{span: span, logger: f.logger}

		if jaegerCtx, ok := span.Context().(jaeger.SpanContext); ok {
			logger.spanFields = []zapcore.Field{
				zap.String("trace_id", jaegerCtx.TraceID().String()),
				zap.String("span_id", jaegerCtx.SpanID().String()),
			}
		}

		return logger
	}
	return f.Default()
}

// With creates a child logger, and optionally adds some context fields to that logger.
func (f Factory) With(fields ...zapcore.Field) Factory {
	return Factory{logger: f.logger.With(fields...)}
}
