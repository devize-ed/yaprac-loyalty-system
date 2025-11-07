package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// queryTracer implements the pgx.Tracer interface to log query execution details.
type queryTracer struct {
	logger *zap.SugaredLogger
}

// TraceQueryStart logs the start of a query execution.
func (t *queryTracer) TraceQueryStart(
	ctx context.Context,
	_ *pgx.Conn,
	data pgx.TraceQueryStartData,
) context.Context {
	t.logger.Debugf("Running query %s (%v)", data.SQL, data.Args)
	return ctx
}

// TraceQueryEnd logs the end of a query execution.
func (t *queryTracer) TraceQueryEnd(_ context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	t.logger.Debugf("%v", data.CommandTag)
}
