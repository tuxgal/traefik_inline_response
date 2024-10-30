package traefik_inline_response_test

import (
	"net/http"
	"net/http/httptest"
)

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		rec: httptest.NewRecorder(),
	}
}

type responseRecorder struct {
	rec       *httptest.ResponseRecorder
	wroteResp bool
}

func (rr *responseRecorder) Flush() {
	rr.wroteResp = true
	rr.rec.Flush()
}

func (rr *responseRecorder) Header() http.Header {
	return rr.rec.Header()
}

func (rr *responseRecorder) Result() *http.Response {
	if !rr.wroteResp {
		return nil
	}
	return rr.rec.Result()
}

func (rr *responseRecorder) Write(buf []byte) (int, error) {
	rr.wroteResp = true
	return rr.rec.Write(buf)
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.wroteResp = true
	rr.rec.WriteHeader(code)
}

func (rr *responseRecorder) WriteString(str string) (int, error) {
	rr.wroteResp = true
	return rr.rec.WriteString(str)
}

func newNextHandler() *nextHandler {
	return &nextHandler{}
}

type nextHandler struct {
	invoked bool
}

func (n *nextHandler) handlerFunc() http.HandlerFunc {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		n.invoked = true
	})
}

func (n *nextHandler) wasInvoked() bool {
	return n.invoked
}
