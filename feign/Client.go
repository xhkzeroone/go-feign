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

// Handler là hàm thực hiện request, nhận các tham số chuẩn hóa
// Bạn có thể mở rộng struct này nếu cần thêm thông tin
// Middleware là hàm nhận next Handler và trả về Handler mới

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

type Client struct {
	*resty.Client
	Config      *Config
	baseURL     string
	headers     map[string]string
	middlewares []Middleware // Thêm trường này
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

// Use cho phép đăng ký middleware vào client
func (c *Client) Use(mw Middleware) {
	c.middlewares = append(c.middlewares, mw)
}

// buildChain xây dựng chuỗi middleware
func (c *Client) buildChain(final Handler) Handler {
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		final = c.middlewares[i](final)
	}
	return final
}

// CallWithoutParam gọi mà không có query param
func (c *Client) CallWithoutParam(ctx context.Context, method, path string, headers map[string]string, body, result interface{}) error {
	return c.CallREST(ctx, method, path, map[string]string{}, map[string]string{}, headers, body, result)
}

// CallWithoutBody gọi mà không có body
func (c *Client) CallWithoutBody(ctx context.Context, method, path string, pathVars, params, headers map[string]string, result interface{}) error {
	return c.CallREST(ctx, method, path, pathVars, params, headers, nil, result)
}

// CallREST là hàm gọi chính
func (c *Client) CallREST(ctx context.Context, method, path string, pathVars, params, headers map[string]string, body, result interface{}) error {
	// Tạo request chuẩn hóa
	req := &Request{
		Context:  ctx,
		Method:   method,
		Path:     path,
		PathVars: pathVars,
		Params:   params,
		Headers:  headers,
		Body:     body,
		Result:   result,
	}

	handler := func(r *Request) error {
		// Format path variables
		p := formatPath(r.Path, r.PathVars)

		reqResty := c.R().SetContext(r.Context)

		// Set global headers
		for k, v := range c.headers {
			reqResty.SetHeader(k, v)
		}

		// Set custom headers
		for k, v := range r.Headers {
			reqResty.SetHeader(k, v)
		}

		// Set query params
		if len(r.Params) > 0 {
			reqResty.SetQueryParams(r.Params)
		}

		// Set body nếu có
		if r.Body != nil {
			reqResty.SetHeader("Content-Type", "application/json")
			reqResty.SetBody(r.Body)
		}

		var resp *resty.Response
		var err error

		switch r.Method {
		case http.MethodGet:
			resp, err = reqResty.Get(p)
		case http.MethodPost:
			resp, err = reqResty.Post(p)
		case http.MethodPut:
			resp, err = reqResty.Put(p)
		case http.MethodDelete:
			resp, err = reqResty.Delete(p)
		default:
			return errors.New("unsupported HTTP method: " + r.Method)
		}
		if err != nil {
			return err
		}

		// Check status
		if !isValidStatus(r.Method, resp.StatusCode()) {
			if c.Config.Debug {
				fmt.Printf("Request failed. Method: %s, URL: %s, Status: %d, Body: %s\n",
					r.Method, p, resp.StatusCode(), string(resp.Body()))
			}
			return errors.New("request failed with status: " + resp.Status())
		}

		if r.Result != nil {
			return json.Unmarshal(resp.Body(), r.Result)
		}
		return nil
	}

	// Nếu có middleware thì chạy qua chain
	if len(c.middlewares) > 0 {
		return c.buildChain(handler)(req)
	}
	// Không có middleware thì chạy logic gốc
	return handler(req)
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

// Download tải file về và ghi vào writer
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

// formatPath thay thế {key} trong path với pathVars[key]
func formatPath(path string, pathVars map[string]string) string {
	for k, v := range pathVars {
		path = strings.ReplaceAll(path, "{"+k+"}", v)
	}
	return path
}

// isValidStatus kiểm tra mã trạng thái có hợp lệ không
func isValidStatus(method string, status int) bool {
	for _, code := range validStatusCodes[method] {
		if status == code {
			return true
		}
	}
	return false
}
