package feign

import "context"

type Request struct {
	Context  context.Context
	Method   string
	Path     string
	PathVars map[string]string
	Params   map[string]string
	Headers  map[string]string
	Body     interface{}
	Result   interface{}
}

type Handler func(req *Request) error

type Middleware func(next Handler) Handler
