package feign

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-resty/resty/v2"
)

var validStatusCodes = map[string][]int{
	http.MethodGet:    {http.StatusOK},
	http.MethodPost:   {http.StatusOK, http.StatusCreated},
	http.MethodPut:    {http.StatusOK},
	http.MethodDelete: {http.StatusOK, http.StatusNoContent},
}

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

type ReqOption struct {
	Context  context.Context
	Method   string
	Path     string
	PathVars map[string]string
	Params   map[string]string
	Headers  map[string]string
	Body     interface{}
}

type Handler func(req *Request) error

type Middleware func(next Handler) Handler

type Client struct {
	*resty.Client
	Config      *Config
	baseURL     string
	headers     map[string]string
	middlewares []Middleware
}

func New(cfg *Config) *Client {
	return &Client{
		baseURL: cfg.Url,
		headers: cfg.Headers,
		Config:  cfg,
		Client: resty.New().
			SetBaseURL(cfg.Url).
			SetTimeout(cfg.Timeout).
			SetRetryCount(cfg.RetryCount).
			SetRetryWaitTime(cfg.RetryWait).
			SetDebug(cfg.Debug).
			OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
				for k, v := range cfg.Headers {
					req.Header.Set(k, v)
				}
				return nil
			}),
	}
}

func Default[T any](cfg *Config, newClient func(*Client) T) T {
	feignClient := New(cfg)
	client := newClient(feignClient)
	feignClient.Create(client)
	return client
}

func (c *Client) Use(mw Middleware) {
	c.middlewares = append(c.middlewares, mw)
}

func (c *Client) buildChain(final Handler) Handler {
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		final = c.middlewares[i](final)
	}
	return final
}

func (c *Client) CallREST(opt ReqOption, result interface{}) error {
	resp, err := c.Execute(opt)
	if err != nil {
		return err
	}
	if result != nil && resp != nil {
		return json.Unmarshal(resp.Body(), result)
	}
	return nil
}

func (c *Client) Execute(opt ReqOption) (*resty.Response, error) {
	req := &Request{
		Context:  opt.Context,
		Method:   opt.Method,
		Path:     opt.Path,
		PathVars: opt.PathVars,
		Params:   opt.Params,
		Headers:  opt.Headers,
		Body:     opt.Body,
	}

	handler := func(r *Request) error {
		p := formatPath(r.Path, r.PathVars)
		reqResty := c.R().SetContext(r.Context)

		for k, v := range c.headers {
			reqResty.SetHeader(k, v)
		}
		for k, v := range r.Headers {
			reqResty.SetHeader(k, v)
		}
		if len(r.Params) > 0 {
			reqResty.SetQueryParams(r.Params)
		}
		if r.Body != nil {
			reqResty.SetHeader("Content-Type", "application/json")
			reqResty.SetBody(r.Body)
		}

		resp, err := reqResty.Execute(r.Method, p)
		if err != nil {
			return err
		}
		r.Result = resp

		if !isValidStatus(r.Method, resp.StatusCode()) {
			if c.Config.Debug {
				fmt.Printf("Request failed: %s %s (%d) => %s\n", r.Method, p, resp.StatusCode(), string(resp.Body()))
			}
			return errors.New("request failed with status: " + resp.Status())
		}
		return nil
	}

	final := handler
	if len(c.middlewares) > 0 {
		final = c.buildChain(handler)
	}

	if err := final(req); err != nil {
		return nil, err
	}
	resp, ok := req.Result.(*resty.Response)
	if !ok || resp == nil {
		return nil, errors.New("unexpected or nil response")
	}
	return resp, nil
}

func (c *Client) CallSOAP(ctx context.Context, path string, soapAction string, body string, result interface{}) error {
	req := c.R().
		SetContext(ctx).
		SetHeader("Content-Type", "text/xml; charset=utf-8").
		SetHeader("SOAPAction", soapAction).
		SetBody(body)

	resp, err := req.Post(path)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return errors.New("SOAP request failed: " + resp.Status())
	}

	if result != nil {
		return xml.Unmarshal(resp.Body(), result)
	}
	return nil
}

func (c *Client) Download(ctx context.Context, path string, writer io.Writer) error {
	resp, err := c.R().
		SetContext(ctx).
		SetDoNotParseResponse(true).
		Get(path)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return errors.New("request failed with status: " + resp.Status())
	}
	defer func(body io.ReadCloser) {
		_ = body.Close()
	}(resp.RawBody())

	_, err = io.Copy(writer, resp.RawBody())
	return err
}

func formatPath(path string, pathVars map[string]string) string {
	for k, v := range pathVars {
		path = strings.ReplaceAll(path, "{"+k+"}", v)
	}
	return path
}

func isValidStatus(method string, status int) bool {
	for _, code := range validStatusCodes[method] {
		if status == code {
			return true
		}
	}
	return false
}
