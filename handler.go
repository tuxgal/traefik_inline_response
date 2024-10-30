package traefik_inline_response

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// Config is the type that holds the configuration for this plugin.
type Config struct {
	Matchers []Matcher `json:"matchers" mapstructure:"matchers"`
	Fallback *Fallback `json:"fallback" mapstructure:"fallback"`
	Debug    bool      `json:"debug" mapstructure:"debug"`
}

type Matcher struct {
	Path       Path     `json:"path" mapstructure:"path"`
	StatusCode *int     `json:"statusCode" mapstructure:"statusCode"`
	Resp       Response `json:"response" mapstructure:"response"`
}

type Path struct {
	Abs    *string `json:"abs" mapstructure:"abs"`
	Prefix *string `json:"prefix" mapstructure:"prefix"`
	Regex  *string `json:"regex" mapstructure:"regex"`
}

type Fallback struct {
	StatusCode *int     `json:"statusCode" mapstructure:"statusCode"`
	Resp       Response `json:"response" mapstructure:"response"`
}

type Response struct {
	Raw      *string         `json:"data" mapstructure:"raw"`
	Template *string         `json:"template" mapstructure:"template"`
	JSON     *map[string]any `json:"json" mapstructure:"json"`
}

const (
	pathMatcherModeUnknown = iota
	pathMatcherModeAbsolutePath
	pathMatcherModePrefix
	pathMatcherModeRegex
)

type pathMatcherMode uint8

const (
	responseModeUnknown = iota
	responseModeEmpty
	responseModeRaw
	responseModeTemplate
	responseModeJSON
)

type responseMode uint8

type handlerRuntime struct {
	matchers []*matcherRuntime
	fallback *fallbackRuntime
}

type matcherRuntime struct {
	path       *pathRuntime
	statusCode int
	resp       *responseRuntime
}

type pathRuntime struct {
	mode   pathMatcherMode
	abs    *string
	prefix *string
	regex  *regexp.Regexp
}

type responseRuntime struct {
	mode  responseMode
	raw   string
	templ *template.Template
	json  string
}

type fallbackRuntime struct {
	statusCode int
	resp       *responseRuntime
}

func CreateConfig() *Config {
	return &Config{
		// Empty for now. Initialize relevant fields if needed in the future.
	}
}

func (c *Config) validate() (*handlerRuntime, error) {
	rt := &handlerRuntime{}
	for _, m := range c.Matchers {
		if m.StatusCode == nil {
			return nil, fmt.Errorf("Must specify a status code in the matcher")
		}

		p, err := validatePath(&m.Path)
		if err != nil {
			return nil, err
		}

		r, err := validateResponse(&m.Resp, "matcher")
		if err != nil {
			return nil, err
		}

		rt.matchers = append(rt.matchers, &matcherRuntime{
			path:       p,
			statusCode: *m.StatusCode,
			resp:       r,
		})
	}

	f, err := validateFallback(c.Fallback)
	if err != nil {
		return nil, err
	}
	rt.fallback = f

	return rt, nil
}

func validatePath(path *Path) (*pathRuntime, error) {
	p := &pathRuntime{}

	if path.Abs != nil {
		if path.Prefix != nil {
			return nil, fmt.Errorf("Cannot specify path prefix when absolute path is specified")
		}
		if path.Regex != nil {
			return nil, fmt.Errorf("Cannot specify path regex when absolute path is specified")
		}
		p.mode = pathMatcherModeAbsolutePath
		p.abs = path.Abs
	} else if path.Prefix != nil {
		if path.Regex != nil {
			return nil, fmt.Errorf("Cannot specify path regex when path prefix is specified")
		}
		p.mode = pathMatcherModePrefix
		p.prefix = path.Prefix
	} else if path.Regex != nil {
		p.mode = pathMatcherModeRegex
		regex, err := regexp.Compile(*path.Regex)
		if err != nil {
			return nil, fmt.Errorf("Invalid regex in matcher path, reason: %w", err)
		}
		p.mode = pathMatcherModeRegex
		p.regex = regex
	} else {
		return nil, fmt.Errorf("At least one of absoltue path, path prefix or path regex must be specified")
	}

	return p, nil
}

func validateResponse(resp *Response, loc string) (*responseRuntime, error) {
	r := &responseRuntime{}

	if resp.Raw != nil {
		if resp.Template != nil {
			return nil, fmt.Errorf("Cannot specify template in %s response when raw is specified", loc)
		}
		if resp.JSON != nil {
			return nil, fmt.Errorf("Cannot specify json in %s response when raw is specified", loc)
		}
		r.mode = responseModeRaw
		r.raw = *resp.Raw
	} else if resp.Template != nil {
		if resp.JSON != nil {
			return nil, fmt.Errorf("Cannot specify json in %s response when template is specified", loc)
		}
		templ, err := template.New("traefik-inline-response").Parse(*resp.Template)
		if err != nil {
			return nil, fmt.Errorf("Invalid template in %s response, reason: %w", loc, err)
		}
		r.mode = responseModeTemplate
		r.templ = templ
	} else if resp.JSON != nil {
		b, err := json.Marshal(*resp.JSON)
		if err != nil {
			return nil, fmt.Errorf("Invalid JSON in %s response, reason: %w", loc, err)
		}
		r.mode = responseModeJSON
		r.json = string(b)
	} else {
		r.mode = responseModeEmpty
	}

	return r, nil
}

func validateFallback(fallback *Fallback) (*fallbackRuntime, error) {
	if fallback == nil {
		return nil, nil
	}

	if fallback.StatusCode == nil {
		return nil, fmt.Errorf("Must specify a status code in the fallback")
	}

	r, err := validateResponse(&fallback.Resp, "fallback")
	if err != nil {
		return nil, err
	}

	return &fallbackRuntime{
		statusCode: *fallback.StatusCode,
		resp:       r,
	}, nil
}

type Handler struct {
	next    http.Handler
	name    string
	runtime *handlerRuntime
}

func prettyPrintJSON(x interface{}) string {
	jsondata, _ := json.MarshalIndent(x, "", "  ")
	return string(jsondata)
}

func log(debug bool, format string, args ...interface{}) {
	if debug {
		os.Stdout.WriteString(fmt.Sprintf("traefik-inline-response - "+format+"\n", args...))
	}
}

func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	log(config.Debug, "received config = %s", prettyPrintJSON(*config))

	rt, err := config.validate()
	if err != nil {
		return nil, err
	}

	return &Handler{
		next:    next,
		name:    name,
		runtime: rt,
	}, nil
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	for _, m := range h.runtime.matchers {
		switch m.path.mode {
		case pathMatcherModeAbsolutePath:
			if req.URL.Path == *m.path.abs {
				respondToRequest(req, writer, m.statusCode, m.resp)
				return
			}
		case pathMatcherModePrefix:
			if strings.HasPrefix(req.URL.Path, *m.path.prefix) {
				respondToRequest(req, writer, m.statusCode, m.resp)
				return
			}
		case pathMatcherModeRegex:
			if m.path.regex.MatchString(req.URL.Path) {
				respondToRequest(req, writer, m.statusCode, m.resp)
				return
			}
		default:
			respondWithError(writer, "invalid path matcher mode, indicating a bug in the plugin")
			return
		}
	}
	if h.runtime.fallback != nil {
		respondToRequest(req, writer, h.runtime.fallback.statusCode, h.runtime.fallback.resp)
		return
	}
	h.next.ServeHTTP(writer, req)
}

func respondToRequest(req *http.Request, writer http.ResponseWriter, statusCode int, resp *responseRuntime) {
	var err error

	switch resp.mode {
	case responseModeEmpty:
		writer.WriteHeader(statusCode)
	case responseModeRaw:
		writer.WriteHeader(statusCode)
		_, err = io.WriteString(writer, resp.raw)
	case responseModeTemplate:
		var buf bytes.Buffer
		err = resp.templ.Execute(&buf, req)
		if err == nil {
			writer.WriteHeader(statusCode)
			_, err = io.Copy(writer, &buf)
		}
	case responseModeJSON:
		writer.WriteHeader(statusCode)
		// TODO: Set the content type header.
		_, err = io.WriteString(writer, resp.json)
	default:
		err = fmt.Errorf("invalid path matcher mode, indicating a bug in the plugin")
	}

	if err != nil {
		respondWithError(writer, fmt.Sprintf("failed while writing the response, reason: %s", err.Error()))
	}
}

func respondWithError(writer http.ResponseWriter, err string) {
	http.Error(writer, err, http.StatusInternalServerError)
}
