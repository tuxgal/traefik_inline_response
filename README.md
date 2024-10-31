# Traefik Inline Response Plugin

[![Build](https://github.com/tuxgal/traefik_inline_response/actions/workflows/build.yml/badge.svg)](https://github.com/tuxgal/traefik_inline_response/actions/workflows/build.yml) [![Tests](https://github.com/tuxgal/traefik_inline_response/actions/workflows/tests.yml/badge.svg)](https://github.com/tuxgal/traefik_inline_response/actions/workflows/tests.yml) [![Lint](https://github.com/tuxgal/traefik_inline_response/actions/workflows/lint.yml/badge.svg)](https://github.com/tuxgal/traefik_inline_response/actions/workflows/lint.yml) [![CodeQL](https://github.com/tuxgal/traefik_inline_response/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/tuxgal/traefik_inline_response/actions/workflows/codeql-analysis.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/tuxgal/traefik_inline_response)](https://goreportcard.com/report/github.com/tuxgal/traefik_inline_response)

A Traefik middleware plugin for responding to requests inline without
a backend.

This allows you to make Traefik act as a request handler with a
custom set of rules that can be configured and without the need for a
backend.

The responses to the requests are configurable based on the path of the
request. The response status code and the body are also configurable.
Responses can be static or dynamic based on the specified template.

## Usage

The [Traefik plugins doc](https://plugins.traefik.io/install) can be used
as the general reference for installing the plugin.

1. Add the plugin to traefik's static configuration:

```yaml
experimental:
  plugins:
    inlineResponse:
      moduleName: github.com/tuxgal/traefik_inline_response
      version: v0.1.2

```

2. Configure the plugin as part of the middleware definition in the
dynamic configuration:

```yaml
http:
  routers:
    to-local-backend:
      rule: 'HostRegexp(`^.*$`)'
      service: local-backend
      middlewares:
        - inline-response
  middlewares:
    inline-response:
      plugin:
        inlineResponse:
          matchers:
            - path:
                abs: /path1
              statusCode: 200
              response:
                raw: Hello from /path1
            - path:
                prefix: /path2
              statusCode: 200
              response:
                json:
                  name: traefik
                  category: plugin
                  health:
                    frontend: ok
                    middleware: ok
                    backend: ok
            - path:
                regex: '^.*/path3/foo/.*$'
              statusCode: 403
              response:
                template: '{{ .Method }}-{{ .URL.Scheme }}-{{ .URL.Host }}-{{ .URL.Path }}'
            - path:
                regex: '^/path4/.+$'
              statusCode: 405
          fallback:
            statusCode: 404
            response:
              template: '{{ .Proto }} {{ .URL.Path }} Not Found'
  services:
    local-backend:
      loadBalancer:
        servers:
          - url: noop@internal
```

Note, how we configure the router to send the request to a service whose
backend is `noop@internal` a special no-op traefik backend. When the plugin
is able to match and respond to the requests, the requests do not reach
this special backend at all.

In the above example, the middleware will respond to the requests based
on the following rules in order without the need for any backend service:

1. Any request paths matching the absolute path `/path1` will return a
   response with `200` status code and body `Hello from /path1`.

2. Any request paths with the prefix `/path2` will return a response with
   `200` status code and the JSON:

```json
{
  "name": "traefik",
  "category": "plugin",
  "health": {
    "frontend": "ok",
    "middleware": "ok",
    "backend": "ok"
  }
}
```

3. Any request paths matching the regular expression `^.*/path3/foo/.*$` will
   return a response with `403` status code and body whose result will be
   the evaluation of the go template `{{ .Method }}-{{ .URL.Scheme }}-{{ .URL.Host }}-{{ .URL.Path }}`
   with the input to the template being the
   [`Request` type from `net/http` package](https://pkg.go.dev/net/http#Request).

4. Any request paths matching the regular expression `^/path4/.+$` will
   return an empty response body with status code `405`.

5. Requests not matching any of the above rules will be handled as per the
   configuration defined under `fallback`. In this case, it will lead to
   a response with status code `404` with the body whose result will be
   the evaluation of the go template `{{ .Proto }} {{ .URL.Path }} Not Found`
   with the input to the template being the
   [`Request` type from `net/http` package](https://pkg.go.dev/net/http#Request).

## Configuration Details

- Every configuration includes a list of path matcher handlers which are
  evaluated in the order defined and an optional fallback handler if the
  request did not match any of the rules specified in the path matcher
  handlers.
- Path matcher handlers are optional.
- Each matcher can match against the request path based on exactly one of
  absolute path, path prefix or a regular expression.
- Response status code is mandatory.
- Response body is optional.
- Response body if specified, can be one of static string, JSON or a go
  template that is evaluated with the request as the input to the template.
- Fallback handler is optional.
- Fallback handler if specified, has the same rules and constraints as
  the response handling configuration specified under a matcher.
- Fallback handler if specified will only handle the request if none of
  the path matcher handlers are able to match the request.
