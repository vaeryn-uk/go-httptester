# httptester

An opinionated API for making assertions against HTTP servers in high-level tests for Golang.

The goal here is to make for succinct yet descriptive tests against an HTTP application. Support
is mainly focussed around JSON APIs.

As with many Go projects, this started out as a package within a personal project, but I moved this
out to reuse elsewhere. Feel free to use it too, but consider it experimental.

[Full docs](https://pkg.go.dev/github.com/vaeryn-uk/go-httptester)

```go
package my_server_test

import (
	"github.com/vaeryn-uk/go-httptester"
	"net/http"
	"testing"
)

func TestMyServer(t *testing.T) {
	// Initialise a handler which implements your HTTP application.
	var serverToTest http.Handler

	// Initialises an HTTP server via httptest.NewServer that will automatically close
	// when the test is ended.
	srv := httptester.Server(t, serverToTest)

	// Create a new tester to assert against it.
	ht := httptester.New(t, srv)

	ht.Request(
		"GET",
		"/",
		// Configure the request further here, e.g. adding a bearer token.
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
}
```