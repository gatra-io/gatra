package evaluator

import (
	"encoding/json"
	"fmt"

	"github.com/gatra-io/gatra/internal/errors"
	"github.com/google/cel-go/cel"
)

// EvaluateCondition compiles and checks a CEL boolean expression against a JSON request body.
func EvaluateCondition(conditionStr string, jsonPayload []byte) (bool, error) {
	if conditionStr == "" {
		return true, nil // No condition configured, automatically passes
	}

	var payloadMap map[string]interface{}
	if err := json.Unmarshal(jsonPayload, &payloadMap); err != nil {
		return false, fmt.Errorf("%w: failed to parse JSON for CEL evaluation: %v", errors.ErrInvalidConfiguration, err)
	}

	env, err := cel.NewEnv(
		cel.Variable("payload", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return false, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	ast, iss := env.Compile(conditionStr)
	if iss.Err() != nil {
		return false, fmt.Errorf("%w: invalid CEL expression '%s': %v", errors.ErrInvalidConfiguration, conditionStr, iss.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		return false, fmt.Errorf("failed to construct CEL program: %w", err)
	}

	out, _, err := prg.Eval(map[string]interface{}{
		"payload": payloadMap,
	})
	if err != nil {
		return false, fmt.Errorf("failed to evaluate CEL expression: %w", err)
	}

	boolVal, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("%w: CEL expression '%s' did not return a boolean", errors.ErrInvalidConfiguration, conditionStr)
	}

	return boolVal, nil
}