package httptester

// Contains some small functions on top of std test's functionality for
// improved failure output.

import (
	"fmt"
	"reflect"
	"strings"
)

// must fatals the test if the provided err is not-nil.
func must(t TestingTB, err error, extra ...any) {
	t.Helper()

	if err != nil {
		fatal(t, err, extra...)
	}
}

// fatal invokes testing.Fatal with some extra formatting on top of the provided args.
func fatal(t TestingTB, failure any, extra ...any) {
	t.Helper()

	out := []string{format(failure)}

	for _, e := range extra {
		out = append(out, format(e))
	}

	t.Fatal(strings.Join(out, "\n"))
}

// equals fatals the test if the provided vals are not equal according to reflect.DeepEqual.
func equals(t TestingTB, expected, actual any, extra ...any) {
	t.Helper()

	if !reflect.DeepEqual(expected, actual) {
		args := []any{"expected", expected, "actual", actual}
		args = append(args, extra...)
		fatal(t, "values are not equal", args...)
	}
}

func format(val any) string {
	return fmt.Sprintf("%v", val)
}
