package traefik_inline_response_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tuxgal/traefik_inline_response"
)

type testRequest struct {
	name   string
	method string
	url    string
	// headers http.Header
	body *string
	want *testResponse
}

type testResponse struct {
	statusCode int
	body       string
	// headers    http.Header
}

var handlerTests = []struct {
	name     string
	config   string
	requests []testRequest
}{
	{
		name:   "Empty Config",
		config: ``,
		requests: []testRequest{
			{
				name:   "",
				method: http.MethodGet,
				url:    "http://localhost/foobarbaz",
				want:   nil,
			},
		},
	},
	{
		name: "Matcher Without Fallback",
		config: `
matchers:
  - path:
      abs: /foo1
    statusCode: 404
    response:
      raw: Not-Found-foo1
  - path:
      prefix: /foobar
    statusCode: 200
    response:
      json:
        f1: v1
        f2: 23
        f3:
          f4: 567
          f5:
            - f6: v2
              f7: v3
            - f8: v4
              f9: 890
  - path:
      regex: '^.*/foo/bar/.*$'
    statusCode: 403
    response:
      template: '{{ .Method }}-{{ .URL.Scheme }}-{{ .URL.Host }}-{{ .URL.Path }}'
  - path:
      regex: '^/foo2/.+$'
    statusCode: 409
`,
		requests: []testRequest{
			{
				name:   "Abs Path Match With Raw Response",
				method: http.MethodGet,
				url:    "http://localhost/foo1",
				want: &testResponse{
					statusCode: http.StatusNotFound,
					body:       "Not-Found-foo1",
				},
			},
			{
				name:   "Prefix Path Match 1 With JSON Response",
				method: http.MethodGet,
				url:    "http://localhost/foobar123",
				want: &testResponse{
					statusCode: http.StatusOK,
					body:       `{"f1":"v1","f2":23,"f3":{"f4":567,"f5":[{"f6":"v2","f7":"v3"},{"f8":"v4","f9":890}]}}`,
				},
			},
			{
				name:   "Prefix Path Match 2 With JSON Response",
				method: http.MethodGet,
				url:    "http://localhost/foobar/rand",
				want: &testResponse{
					statusCode: http.StatusOK,
					body:       `{"f1":"v1","f2":23,"f3":{"f4":567,"f5":[{"f6":"v2","f7":"v3"},{"f8":"v4","f9":890}]}}`,
				},
			},
			{
				name:   "Regex Path Match 1 With Template Response",
				method: http.MethodGet,
				url:    "http://localhost/foo/bar/",
				want: &testResponse{
					statusCode: http.StatusForbidden,
					body:       "GET-http-localhost-/foo/bar/",
				},
			},
			{
				name:   "Regex Path Match 2 With Template Response",
				method: http.MethodGet,
				url:    "http://localhost/abc/foo/bar/def",
				want: &testResponse{
					statusCode: http.StatusForbidden,
					body:       "GET-http-localhost-/abc/foo/bar/def",
				},
			},
			{
				name:   "Regex Path Match 3 With Template Response",
				method: http.MethodGet,
				url:    "http://localhost/foo/bar/pqrs",
				want: &testResponse{
					statusCode: http.StatusForbidden,
					body:       "GET-http-localhost-/foo/bar/pqrs",
				},
			},
			{
				name:   "Regex Path Match 4 With Template Response",
				method: http.MethodGet,
				url:    "http://localhost/xyz/foo/bar/",
				want: &testResponse{
					statusCode: http.StatusForbidden,
					body:       "GET-http-localhost-/xyz/foo/bar/",
				},
			},
			{
				name:   "Regex Path Match With Empty Response",
				method: http.MethodGet,
				url:    "http://localhost/foo2/bar/",
				want: &testResponse{
					statusCode: http.StatusConflict,
					body:       "",
				},
			},
			{
				name:   "No Match With No Response",
				method: http.MethodGet,
				url:    "http://localhost/foo3",
				want:   nil,
			},
		},
	},
	{
		name: "Matcher With Fallback",
		config: `
matchers:
  - path:
      abs: /foo1
    statusCode: 404
    response:
      raw: Not-Found-foo1
  - path:
      prefix: /foobar
    statusCode: 200
    response:
      json:
        f1: v1
        f2: 23
        f3:
          f4: 567
          f5:
            - f6: v2
              f7: v3
            - f8: v4
              f9: 890
  - path:
      regex: '^.*/foo/bar/.*$'
    statusCode: 403
    response:
      template: '{{ .Method }}-{{ .URL.Scheme }}-{{ .URL.Host }}-{{ .URL.Path }}'
  - path:
      regex: '^/foo2/.+$'
    statusCode: 409
fallback:
  statusCode: 204
  response:
    template: '{{ .Proto }}-{{ .URL.Path }}'
`,
		requests: []testRequest{
			{
				name:   "Fallback Match",
				method: http.MethodGet,
				url:    "http://localhost/foo3",
				want: &testResponse{
					statusCode: http.StatusNoContent,
					body:       "HTTP/1.1-/foo3",
				},
			},
		},
	},
	{
		name: "Error Response",
		config: `
matchers:
  - path:
      abs: /foo1
    statusCode: 200
    response:
      template: '{{ .Method }}-{{ .URL.Scheme }}-{{ .garbage }}'
`,
		requests: []testRequest{
			{
				name:   "Template Execution Error Response",
				method: http.MethodGet,
				url:    "http://localhost/foo1",
				want: &testResponse{
					statusCode: http.StatusInternalServerError,
					body: `failed while writing the response, reason: template: traefik-inline-response:1:35: executing "traefik-inline-response" at <.garbage>: can't evaluate field garbage in type *http.Request
`,
				},
			},
		},
	},
}

func TestHandler(t *testing.T) {
	t.Parallel()

	for _, test := range handlerTests {
		tc := test
		ctx := context.Background()
		next := newNextHandler()
		config := buildConfig(tc.config)

		handler, err := traefik_inline_response.New(ctx, next.handlerFunc(), config, "inline-response")
		if err != nil {
			logTestFail(t, tc.name, "failed to initialize handler, reason: %v", err)
			return
		}

		for _, input := range tc.requests {
			tcName := fmt.Sprintf("%s - %s", tc.name, input.name)
			t.Run(tcName, func(t *testing.T) {
				rec := newResponseRecorder()
				var body io.Reader
				if input.body != nil {
					body = strings.NewReader(*input.body)
				}
				req, err := http.NewRequestWithContext(ctx, input.method, input.url, body)
				if err != nil {
					logTestFail(t, tcName, "failed to initialize request, reason: %v", err)
					return
				}

				handler.ServeHTTP(rec, req)
				result := rec.Result()
				want := input.want

				if result == nil && want != nil {
					logTestFail(t, tcName, "Did not receive response for request when it was expected")
					return
				}
				if result != nil && want == nil {
					logTestFail(t, tcName, "received response %v for request when it was unexpected", result)
					return
				}
				if next.wasInvoked() && want != nil {
					logTestFail(t, tcName, "next handler was invoked when response was expected from inline response handler")
					return
				}
				if !next.wasInvoked() && want == nil {
					logTestFail(t, tcName, "next handler was not invoked when response was not expected from inline response handler")
					return
				}

				if want != nil {
					gotStatusCode := result.StatusCode
					if gotStatusCode != want.statusCode {
						logTestFail(t, tcName, "got status code %d does not match the want status code %d", gotStatusCode, want.statusCode)
						return
					}

					gotBody, err := readBody(result.Body)
					if err != nil {
						logTestFail(t, tcName, "failed to read body, reason: %v", err)
						return
					}
					if gotBody != want.body {
						logTestFail(t, tcName, "got != want in response body\ngot:  %s\nwant: %s\n", gotBody, want.body)
						return
					}
				}
			})
		}
	}
}

var handlerValidationErrorTests = []struct {
	name   string
	config string
	want   string
}{
	{
		name: "Matcher Without Status Code",
		config: `
matchers:
  - path:
      abs: /foo
`,
		want: `Must specify a status code in the matcher`,
	},
	{
		name: "Matcher With Both Absolute Path And Path Prefix",
		config: `
matchers:
  - path:
      abs: /foo
      prefix: /bar
    statusCode: 404
`,
		want: `Cannot specify path prefix when absolute path is specified`,
	},
	{
		name: "Matcher With Both Absolute Path And Path Regex",
		config: `
matchers:
  - path:
      abs: /foo
      regex: '^.+$'
    statusCode: 404
`,
		want: `Cannot specify path regex when absolute path is specified`,
	},
	{
		name: "Matcher With Both Path Prefix And Path Regex",
		config: `
matchers:
  - path:
      prefix: /foo
      regex: '^.+$'
    statusCode: 404
`,
		want: `Cannot specify path regex when path prefix is specified`,
	},
	{
		name: "Matcher With No Path",
		config: `
matchers:
  - path: {}
    statusCode: 404
`,
		want: `At least one of absoltue path, path prefix or path regex must be specified`,
	},
	{
		name: "Matcher With Invalid Path Regex",
		config: `
matchers:
  - path:
      regex: '*'
    statusCode: 404
`,
		want: "Invalid regex in matcher path, reason: error parsing regexp: missing argument to repetition operator: `*`",
	},
	{
		name: "Matcher Response With Both Raw And Template",
		config: `
matchers:
  - path:
      abs: '/foo'
    response:
      raw: OK
      template: '{{ .URL.Path }}'
    statusCode: 404
`,
		want: `Cannot specify template in matcher response when raw is specified`,
	},
	{
		name: "Matcher Response With Both Raw And JSON",
		config: `
matchers:
  - path:
      abs: '/foo'
    response:
      raw: OK
      json:
        abc: def
    statusCode: 404
`,
		want: `Cannot specify json in matcher response when raw is specified`,
	},
	{
		name: "Matcher Response With Both Template And JSON",
		config: `
matchers:
  - path:
      abs: '/foo'
    response:
      template: '{{ .URL.Path }}'
      json:
        abc: def
    statusCode: 404
`,
		want: `Cannot specify json in matcher response when template is specified`,
	},
	{
		name: "Matcher Response With Invalid Template",
		config: `
matchers:
  - path:
      abs: '/foo'
    response:
      template: '{{ .URL.Path'
    statusCode: 404
`,
		want: `Invalid template in matcher response, reason: template: traefik-inline-response:1: unclosed action`,
	},
	{
		name: "Fallback Without Status Code",
		config: `
fallback: {}
`,
		want: `Must specify a status code in the fallback`,
	},
	{
		name: "Fallback Response With Both Raw And Template",
		config: `
fallback:
  response:
    raw: OK
    template: '{{ .URL.Path }}'
  statusCode: 404
`,
		want: `Cannot specify template in fallback response when raw is specified`,
	},
	{
		name: "Fallback Response With Both Raw And JSON",
		config: `
fallback:
  response:
    raw: OK
    json:
      abc: def
  statusCode: 404
`,
		want: `Cannot specify json in fallback response when raw is specified`,
	},
	{
		name: "Fallback Response With Both Template And JSON",
		config: `
fallback:
  response:
    template: '{{ .URL.Path }}'
    json:
      abc: def
  statusCode: 404
`,
		want: `Cannot specify json in fallback response when template is specified`,
	},
	{
		name: "Fallback Response With Invalid Template",
		config: `
fallback:
  response:
    template: '{{ .URL.Path'
  statusCode: 404
`,
		want: `Invalid template in fallback response, reason: template: traefik-inline-response:1: unclosed action`,
	},
}

func TestHandlerValidationErrors(t *testing.T) {
	t.Parallel()

	for _, test := range handlerValidationErrorTests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			next := newNextHandler()
			config := buildConfig(tc.config)

			_, err := traefik_inline_response.New(ctx, next.handlerFunc(), config, "inline-response")
			if err == nil {
				logTestFail(t, tc.name, "did not receive any errors after config validation when one was expected")
				return
			}

			got := err.Error()
			if got != tc.want {
				logTestFail(t, tc.name, "got != want in config validation error received\ngot:  %s\nwant: %s\n", got, tc.want)
				return
			}
		})
	}
}

func readBody(data io.ReadCloser) (string, error) {
	defer data.Close()

	b, err := io.ReadAll(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func logTestFail(t *testing.T, testCase string, fmt string, args ...interface{}) {
	t.Helper()
	var a []interface{}
	a = append(a, testCase)
	a = append(a, args...)
	t.Errorf("\nTest Case: %q\nReason: "+fmt, a...)
}
