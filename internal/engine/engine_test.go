package engine_test

import (
	"io"
	"log/slog"
	"testing"

	"github.com/gatra-io/gatra/internal/config"
	"github.com/gatra-io/gatra/internal/engine"
	"github.com/gatra-io/gatra/internal/evaluator"
)

func BenchmarkEvaluateAndIncrement(b *testing.B) {
	noopLogger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	eng, err := engine.NewEngine(0, noopLogger, nil)
	if err != nil {
		b.Fatalf("failed to init engine: %v", err)
	}

	limit := 1000000000.0
	rule := config.Rule{
		RuleID:      "bench_rule",
		ToolPattern: "stripe/refund",
		Limits:      config.Limits{MaxCumulative: &limit},
		Accumulator: config.Accumulator{Type: config.AccumulatorSum},
	}

	val := 1.0
	evalRes := &evaluator.EvaluationResult{NumericValue: &val}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = eng.EvaluateAndIncrement("bench_traj_001", rule, evalRes)
		}
	})
}