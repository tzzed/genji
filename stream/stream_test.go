package stream_test

import (
	"testing"

	"github.com/genjidb/genji/document"
	"github.com/genjidb/genji/sql/parser"
	"github.com/genjidb/genji/sql/query/expr"
	"github.com/genjidb/genji/stream"
	"github.com/stretchr/testify/require"
)

func TestStream(t *testing.T) {
	s := stream.New(stream.NewValueIterator(
		document.NewIntegerValue(1),
		document.NewIntegerValue(2),
	))

	s = s.Pipe(stream.Map(parser.MustParseExpr("_v + 1")))
	s = s.Pipe(stream.Filter(parser.MustParseExpr("_v > 2")))

	var count int64
	err := s.Iterate(func(env *expr.Environment) error {
		v, ok := env.GetCurrentValue()
		require.True(t, ok)
		require.Equal(t, document.NewIntegerValue(count+3), v)
		count++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}
