# Traefik Inline Response Plugin

[![Build](https://github.com/tuxgal/traefik_inline_response/actions/workflows/build.yml/badge.svg)](https://github.com/tuxgal/traefik_inline_response/actions/workflows/build.yml) [![Tests](https://github.com/tuxgal/traefik_inline_response/actions/workflows/tests.yml/badge.svg)](https://github.com/tuxgal/traefik_inline_response/actions/workflows/tests.yml) [![Lint](https://github.com/tuxgal/traefik_inline_response/actions/workflows/lint.yml/badge.svg)](https://github.com/tuxgal/traefik_inline_response/actions/workflows/lint.yml) [![CodeQL](https://github.com/tuxgal/traefik_inline_response/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/tuxgal/traefik_inline_response/actions/workflows/codeql-analysis.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/tuxgal/traefik_inline_response)](https://goreportcard.com/report/github.com/tuxgal/traefik_inline_response)

A Traefik middleware plugin for responding to requests inline without
a backend.

The responses to the requests are configurable based on the path of the
request. The response status code and the body are also configurable.
Responses can be static or dynamic based on the specified template.
