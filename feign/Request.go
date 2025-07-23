package feign

import (
	"context"
	"net/http"
)

type ReqOption interface {
	Context() context.Context
	Method() string
	Path() string
	PathVars() map[string]string
	Params() map[string]string
	Headers() map[string]string
	Body() interface{}
}

type reqOption struct {
	ctx      context.Context
	method   string
	path     string
	pathVars map[string]string
	params   map[string]string
	headers  map[string]string
	body     interface{}
}

func (r *reqOption) Context() context.Context {
	return r.ctx
}

func (r *reqOption) Method() string {
	return r.method
}

func (r *reqOption) Path() string {
	return r.path
}

func (r *reqOption) PathVars() map[string]string {
	return r.pathVars
}

func (r *reqOption) Params() map[string]string {
	return r.params
}

func (r *reqOption) Headers() map[string]string {
	return r.headers
}

func (r *reqOption) Body() interface{} {
	return r.body
}

type ReqOptionBuilder struct {
	opt *reqOption
}

func NewRequest() *ReqOptionBuilder {
	return &ReqOptionBuilder{
		opt: &reqOption{
			pathVars: make(map[string]string),
			params:   make(map[string]string),
			headers:  make(map[string]string),
		},
	}
}

func (b *ReqOptionBuilder) WithContext(ctx context.Context) *ReqOptionBuilder {
	b.opt.ctx = ctx
	return b
}

func (b *ReqOptionBuilder) MethodPost() *ReqOptionBuilder {
	b.opt.method = http.MethodPost
	return b
}

func (b *ReqOptionBuilder) MethodGet() *ReqOptionBuilder {
	b.opt.method = http.MethodGet
	return b
}

func (b *ReqOptionBuilder) MethodPut() *ReqOptionBuilder {
	b.opt.method = http.MethodPut
	return b
}

func (b *ReqOptionBuilder) MethodDelete() *ReqOptionBuilder {
	b.opt.method = http.MethodDelete
	return b
}

func (b *ReqOptionBuilder) WithMethod(method string) *ReqOptionBuilder {
	b.opt.method = method
	return b
}

func (b *ReqOptionBuilder) WithPath(path string) *ReqOptionBuilder {
	b.opt.path = path
	return b
}

func (b *ReqOptionBuilder) AddPathVar(key, value string) *ReqOptionBuilder {
	b.opt.pathVars[key] = value
	return b
}

func (b *ReqOptionBuilder) AddParam(key, value string) *ReqOptionBuilder {
	b.opt.params[key] = value
	return b
}

func (b *ReqOptionBuilder) AddHeader(key, value string) *ReqOptionBuilder {
	b.opt.headers[key] = value
	return b
}

func (b *ReqOptionBuilder) WithBody(body interface{}) *ReqOptionBuilder {
	b.opt.body = body
	return b
}

func (b *ReqOptionBuilder) Build() ReqOption {
	return b.opt
}
