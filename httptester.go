package httptester

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/vaeryn-uk/frostember-server/pkg/fbrmath"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"runtime/debug"
	"strings"
)

// TestingTB is a subset of testing.TB. This is here to allow
// for example code, but in real tests, this anything that accepts
// TestingTB should be given a testing.TB.
type TestingTB interface {
	Cleanup(func())
	Helper()
	Fatal(args ...any)
	Log(arg ...any)
}

// Server starts and returns a new httptest.Server which will shutdown with the
// test.
func Server(t TestingTB, handler http.Handler) *httptest.Server {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// HttpTester offers a convenient API for high-level HTTP testing.
// Use New to get one.
type HttpTester struct {
	t             TestingTB
	srv           *httptest.Server
	client        *http.Client
	requests      []*HttpTesterRequest
	multipartForm *multipart.Writer
}

// New creates a new HttpTester wrapping t and using srv.
// Common usage:
//
//	ht := NewHttpTester(t, srv)
//	ht.Request("GET", "/api/test", ht.SomeOption(), ...).Expect(ht.SomeExpectation(), ...).Test()
func New(t TestingTB, srv *httptest.Server) *HttpTester {
	tester := &HttpTester{
		t:        t,
		srv:      srv,
		client:   srv.Client(),
		requests: make([]*HttpTesterRequest, 0),
	}

	t.Cleanup(func() {
		for _, req := range tester.requests {
			if !req.done {
				fatal(t, fmt.Sprintf("forgot to execute Test on test request at:\n%s\n", string(req.stack)))
			}
		}
	})

	return tester
}

// RequestOption is used to configure an HttpTesterRequest.
type RequestOption func(req *HttpTesterRequest)

// Request creates a configured HttpTesterRequest. Forgetting to call Expect().Test() on this
// request will cause a failure in the test.
func (h *HttpTester) Request(method, path string, options ...RequestOption) *HttpTesterRequest {
	req, err := http.NewRequest(method, path, nil)
	must(h.t, err)

	request := &HttpTesterRequest{
		request: req,
		tester:  h,
		stack:   debug.Stack(),
	}

	h.requests = append(h.requests, request)

	for _, opt := range options {
		opt(request)
	}

	return request
}

// Bearer configures a HttpTesterRequest with an bearer authorization token.
func (h *HttpTester) Bearer(token string) RequestOption {
	return h.Header("Authorization", fmt.Sprintf("Bearer %s", token))
}

// Header configures a HttpTesterRequest to have a have with the given name, set to the
// given val. E.g.:
//
//	ht.Header("Content-Type", "text/plain")
func (h *HttpTester) Header(name, val string) RequestOption {
	return func(req *HttpTesterRequest) {
		req.request.Header.Set(name, val)
	}
}

// JsonBody configures a HttpTesterRequest with a JSON body. Supports format
// parameters. If body is a reader, will grab the string data from that. If it
// is any other non-string body, we json.Unmarshal it to get a string.
func (h *HttpTester) JsonBody(body any, args ...any) RequestOption {
	var bodyStr string

	if asReader, isReader := body.(io.Reader); isReader {
		bodyBytes, err := io.ReadAll(asReader)
		must(h.t, err)
		bodyStr = string(bodyBytes)
	}

	var isStr bool

	bodyStr, isStr = body.(string)

	if !isStr {
		b, err := json.MarshalIndent(body, "", "  ")
		must(h.t, err, "cannot convert body data to JSON", body)
		bodyStr = string(b)
	}

	return func(req *HttpTesterRequest) {
		req.request.Header.Set("Content-Type", "application/json")
		req.request.Body = io.NopCloser(strings.NewReader(fmt.Sprintf(bodyStr, args...)))
	}
}

func (h *HttpTester) MultipartFormFile(fieldname, filename string, data io.Reader) RequestOption {
	return func(req *HttpTesterRequest) {
		file, err := req.multipart().CreateFormFile(fieldname, filename)
		must(h.t, err)

		_, err = io.Copy(file, data)
		must(h.t, err)
	}
}

func (h *HttpTester) MultipartFormField(name string, val []byte) RequestOption {
	return func(req *HttpTesterRequest) {
		field, err := req.multipart().CreateFormField(name)
		must(h.t, err)

		_, err = field.Write(val)
		must(h.t, err)
	}
}

// ExpectCode configures an HttpExpectation to require a certain response code.
func (h *HttpTester) ExpectCode(code int) ResponseOption {
	return func(expectation *HttpExpectation) {
		expectation.addExpectation(func(response *http.Response, body string, extra ...any) {
			h.t.Helper()
			equals(h.t, code, response.StatusCode, extra...)
		})
	}
}

// ExpectBodyContains configure an HttpExpectation to require the response body
// contains the content string at least once.
func (h *HttpTester) ExpectBodyContains(content string) ResponseOption {
	return func(expectation *HttpExpectation) {
		expectation.addExpectation(func(response *http.Response, body string, extra ...any) {
			if strings.Index(body, content) < 0 {
				args := []any{"contains", content, "body", body}
				args = append(args, extra...)
				fatal(h.t, "body contains failed", args...)
			}
		})
	}
}

func (h *HttpTester) ExpectContentType(contentType string) ResponseOption {
	return func(expectation *HttpExpectation) {
		expectation.addExpectation(func(response *http.Response, body string, extra ...any) {
			equals(h.t, contentType, response.Header.Get("Content-Type"), extra...)
		})
	}
}

// ExpectJsonExists configures an HttpExpectation to require a JSON body which contains
// a non-empty string value at jsonpath path.
func (h *HttpTester) ExpectJsonExists(path string) ResponseOption {
	h.t.Helper()

	return func(expectation *HttpExpectation) {
		expectation.addExpectation(func(response *http.Response, body string, extra ...any) {
			h.t.Helper()

			JsonContainsStr(h.t, body, path, extra...)
		})
	}
}

// ExpectJsonMatchStr extends ExpectJsonExists to also ensure that the value found at jsonpath
// path matches the expected string match.
func (h *HttpTester) ExpectJsonMatchStr(path, match string) ResponseOption {
	return func(expectation *HttpExpectation) {
		expectation.addExpectation(func(response *http.Response, body string, extra ...any) {
			h.t.Helper()

			extra = append([]any{fmt.Sprintf("json path: %s", path)}, extra...)
			equals(h.t, match, JsonContainsStr(h.t, body, path, extra...), extra...)
		})
	}
}

// ExpectJsonMatch asserts that the HTTP response has a JSON body which contains a value
// at JSON path which matches parameter match.
// Note that numbers in the JSON will be float64 in Go.
func (h *HttpTester) ExpectJsonMatch(path string, match any) ResponseOption {
	return func(expectation *HttpExpectation) {
		expectation.addExpectation(func(response *http.Response, body string, extra ...any) {
			h.t.Helper()

			extra = append([]any{fmt.Sprintf("json path: %s", path)}, extra...)
			equals(h.t, match, JsonContains(h.t, body, path, extra...), extra...)
		})
	}
}

// CaptureJson defines a capture against the response's JSON body. If
// successful, this capture is available under name from HttpExpectation.Test.
// Will fatal if there are no string value to capture, so this implies ExpectJsonExists.
func (h *HttpTester) CaptureJson(name, jsonpath string) ResponseOption {
	return func(expectation *HttpExpectation) {
		h.t.Helper()

		expectation.jsonCaptures[name] = jsonpath
	}
}

// HttpTesterRequest defines a request we're going to test against.
type HttpTesterRequest struct {
	request             *http.Request
	tester              *HttpTester
	done                bool
	stack               []byte
	multipartForm       *multipart.Writer
	multipartFormBuffer *bytes.Buffer
}

// Expect returns a configured HttpExpectation to test against.
func (h *HttpTesterRequest) Expect(options ...ResponseOption) *HttpExpectation {
	expectation := &HttpExpectation{
		request:              h,
		responseExpectations: make([]responseExpectation, 0),
		jsonCaptures:         make(map[string]string),
	}

	for _, opt := range options {
		opt(expectation)
	}

	return expectation
}

// ResponseOption is used to configure an HttpExpectation.
type ResponseOption func(expectation *HttpExpectation)

type responseExpectation func(response *http.Response, body string, extra ...any)

// HttpExpectation defines what we expect to receive after sending an
// HttpTesterRequest, plus any data we want to pull out of it.
type HttpExpectation struct {
	request              *HttpTesterRequest
	responseExpectations []responseExpectation
	jsonCaptures         map[string]string
}

func (h *HttpExpectation) addExpectation(expectation responseExpectation) {
	h.responseExpectations = append(h.responseExpectations, expectation)
}

func (h *HttpTesterRequest) multipart() *multipart.Writer {
	if h.multipartForm == nil {
		h.multipartFormBuffer = &bytes.Buffer{}
		h.multipartForm = multipart.NewWriter(h.multipartFormBuffer)
	}

	return h.multipartForm
}

func (h *HttpTesterRequest) finalise() *http.Request {
	if h.multipartForm == nil {
		return h.request
	}

	// Finish and attach a multipart form if we have started one.
	must(h.tester.t, h.multipartForm.Close())
	h.request.Body = io.NopCloser(h.multipartFormBuffer)

	h.request.Header.Set("Content-Type", h.multipartForm.FormDataContentType())

	return h.request
}

// MaxReqRespOutput is used when reporting test failures. The maximum amount
// of request or response output is printed.
var MaxReqRespOutput = 1200

// Test executes the associated request, failing if expectations are not met,
// else applies any captures.
func (h *HttpExpectation) Test(extra ...any) (captures map[string]string) {
	h.request.tester.t.Helper()

	h.request.done = true

	r := h.request.finalise()
	srv := h.request.tester.srv
	t := h.request.tester.t

	var err error
	r.URL, err = r.URL.Parse(srv.URL + r.URL.String())
	must(t, err, extra...)

	if reqData, err := httputil.DumpRequest(r, true); err == nil {
		l, _ := fbrmath.Min(MaxReqRespOutput, len(reqData))
		extra = append(extra, "HTTP request:", string(reqData[0:l]))
	}

	resp, err := h.request.tester.client.Do(r)
	must(t, err, extra...)

	body, err := io.ReadAll(resp.Body)
	must(t, err, extra...)

	// Replace the body so it can be read again.
	must(t, resp.Body.Close())
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	bodyStr := string(body)

	if respData, err := httputil.DumpResponse(resp, true); err == nil {
		l, _ := fbrmath.Min(MaxReqRespOutput, len(respData))
		extra = append(extra, "HTTP response:", string(respData[0:l]))
	} else {
		t.Log(err)
	}

	for _, expectation := range h.responseExpectations {
		expectation(resp, bodyStr, extra...)
	}

	captures = make(map[string]string)

	for name, expr := range h.jsonCaptures {
		captures[name] = JsonContainsStr(t, bodyStr, expr, extra...)
	}

	return captures
}
