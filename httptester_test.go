package httptester_test

import (
	"encoding/json"
	"fmt"
	"github.com/vaeryn-uk/go-httptester"
	"net/http"
)

var exampleJson = []map[string]any{
	{
		"name": "Scotty",
		"address": map[string]any{
			"number": "123",
			"street": "Fake Street",
			"city":   "Cloud City",
			"zip":    "71622",
		},
	},
}

func ExampleHttpTester() {
	// For this example, our handler always returns some static JSON.
	handler := exampleHttpHandler()

	// In your tests, t will be given to you. For this example, this just prints
	// any test failures.
	t := &exampleTestRunner{}

	// Initialises an HTTP server via httptest.NewServer that will automatically close
	// when the test is ended.
	srv := httptester.Server(t, handler)

	// Create a new tester to assert against it.
	ht := httptester.New(t, srv)

	ht.Request(
		"GET",
		"/",
		// Configure the request here, e.g. adding a bearer token.
		ht.Bearer("some access token"),
		// Or a JSON body.
		ht.JsonBody("some data"),
	).Expect(
		// Use ExpectXXX() to make assertions against it.
		// Like its return code.
		ht.ExpectCode(200),
		// Or that the raw body contains some string.
		ht.ExpectBodyContains("Fake Street"),
		// Or that a jsonpath expression exists
		ht.ExpectJsonExists("$[0].name"),
		// Or that a jsonpath expression resolve to some specific value
		ht.ExpectJsonMatchStr("$[0].name", "Scotty"),
	).
		// Finally invoke Test() to perform the test.
		Test("optional additional info here will be printed on test failure")

	// You can also use captures to extract JSON values from the response.
	ht = httptester.New(t, srv)
	captures := ht.Request("GET", "/").
		Expect(ht.CaptureJson("street", "$[0].address.street")).
		Test()

	// Now we can use the captured value "street" for other things.
	fmt.Println("captured value:", captures["street"])

	// An example of a failing test.
	ht = httptester.New(t, srv)
	ht.Request("GET", "/").Expect(ht.ExpectJsonExists("$[0].foo")).Test()

	// Output:
	// captured value: Fake Street
	// TEST FATAL: unknown key foo
	// failed to capture JSON path
	// $[0].foo
	// full data
	// [{"address":{"city":"Cloud City","number":"123","street":"Fake Street","zip":"71622"},"name":"Scotty"}]
}

// exampleHttpHandler creates a test handler to demonstrate the httptester API.
// This just always replies with some JSON.
func exampleHttpHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		out, _ := json.Marshal(exampleJson)

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(out)
	})
}

// used to satisfy a testing.TB within our example code.
type exampleTestRunner struct {
	finished bool
}

func (e *exampleTestRunner) Cleanup(f func()) {
	// Nothing to do.
}

func (e *exampleTestRunner) Helper() {
	// Nothing to do.
}

func (e *exampleTestRunner) Fatal(args ...any) {
	if !e.finished {
		args = append([]any{"TEST FATAL:"}, args...)
		fmt.Println(args...)
		e.finished = true
	}
}

func (e *exampleTestRunner) Log(args ...any) {
	fmt.Println(args...)
}
