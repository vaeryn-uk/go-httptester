package httptester

import (
	"context"
	"encoding/json"
	"github.com/PaesslerAG/gval"
	"github.com/PaesslerAG/jsonpath"
	"io"
	"strings"
)

// JsonContainsStr fatals the test if the provided JSON data does not contain a string value
// at pathexpr, as per JSONPath.
// https://www.ietf.org/archive/id/draft-goessner-dispatch-jsonpath-00.html
// Returns the resolved string.
func JsonContainsStr(t TestingTB, data string, pathexpr string, extra ...any) string {
	captured := JsonContains(t, data, pathexpr, extra...)

	var capturedStr string
	capturedStr, isStr := captured.(string)
	if !isStr {
		args := []any{"path", pathexpr, "val", captured, "full data", data}
		args = append(args, extra...)
		fatal(t, "jsonpath does not resolve to a string value", args...)
	}

	return capturedStr
}

// JsonContains fatals the test if the provided JSON data does not contain a value
// at pathexpr, as per JSONPath.
// https://www.ietf.org/archive/id/draft-goessner-dispatch-jsonpath-00.html
// Returns the resolved value.
func JsonContains(t TestingTB, data string, pathexpr string, extra ...any) any {
	t.Helper()

	builder := gval.Full(jsonpath.PlaceholderExtension())

	path, err := builder.NewEvaluable(pathexpr)
	must(t, err, extra...)

	body := MustParseJson[any](t, strings.NewReader(data), extra...)

	captured, err := path(context.Background(), body)
	must(t, err, "failed to capture JSON path", pathexpr, "full data", data)

	return captured
}

// MustParseJson will fatal the test if in cannot be decoded. Returns the decoded
// item.
func MustParseJson[T any](t TestingTB, in io.Reader, extra ...any) T {
	t.Helper()

	byteData, err := io.ReadAll(in)
	must(t, err, extra...)

	if len(byteData) == 0 {
		fatal(t, "cannot JSON parse an empty string", extra...)
	}

	var out T
	err = json.Unmarshal(byteData, &out)
	must(t, err, extra...)

	return out
}
