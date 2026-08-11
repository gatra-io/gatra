package evaluator

import (
	"fmt"
	"path"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/gatra-io/gatra/internal/config"
	"github.com/gatra-io/gatra/internal/errors"
)

// EvaluationResult contains the extracted target value and matching rule metadata.
type EvaluationResult struct {
	RuleID         string
	NumericValue   *float64
	StringValue    *string
	StringArrayVal []string
}

// normalizePath converts standard JSONPath expressions (e.g., "$.amount") into gjson format ("amount").
func normalizePath(path string) string {
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, "$")
	return path
}

// MatchPattern tests if an incoming tool name matches a wildcard rule pattern (e.g., "aws/s3/*").
func MatchPattern(pattern, tool string) bool {
	matched, err := path.Match(pattern, tool)
	if err != nil {
		return false
	}
	return matched
}

// ExtractValue parses raw JSON payload data and extracts values defined by valuePath.
func ExtractValue(jsonPayload []byte, valuePath string) (*EvaluationResult, error) {
	if len(jsonPayload) == 0 {
		return nil, fmt.Errorf("%w: empty request payload", errors.ErrInvalidConfiguration)
	}

	cleanPath := normalizePath(valuePath)
	result := gjson.GetBytes(jsonPayload, cleanPath)
	if !result.Exists() {
		return nil, fmt.Errorf("%w: path '%s' not found in payload", errors.ErrInvalidConfiguration, valuePath)
	}

	res := &EvaluationResult{}

	switch result.Type {
	case gjson.Number:
		val := result.Float()
		res.NumericValue = &val

	case gjson.String:
		val := result.String()
		res.StringValue = &val

	case gjson.JSON:
		if result.IsArray() {
			var arr []string
			result.ForEach(func(_, value gjson.Result) bool {
				arr = append(arr, value.String())
				return true
			})
			res.StringArrayVal = arr
		} else {
			return nil, fmt.Errorf("%w: path '%s' resolved to complex JSON object", errors.ErrInvalidConfiguration, valuePath)
		}

	default:
		return nil, fmt.Errorf("%w: unsupported JSON value type at path '%s'", errors.ErrInvalidConfiguration, valuePath)
	}

	return res, nil
}

// EvaluateRule verifies stateless per-call constraints against an extracted result.
func EvaluateRule(rule config.Rule, res *EvaluationResult) error {
	if rule.Limits.MaxPerCall == nil {
		return nil
	}

	if res.NumericValue != nil {
		if *res.NumericValue > *rule.Limits.MaxPerCall {
			return errors.NewProxyError(
				403,
				rule.RuleID,
				fmt.Errorf("%w: value %f exceeds per-call limit %f",
					errors.ErrPerCallLimitExceeded, *res.NumericValue, *rule.Limits.MaxPerCall),
			)
		}
	}

	return nil
}