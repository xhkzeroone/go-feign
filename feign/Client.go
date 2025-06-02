package feign

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"io"
	"net/http"
	"strings"
)

var validStatusCodes = map[string][]int{
	http.MethodGet:    {http.StatusOK},
	http.MethodPost:   {http.StatusOK, http.StatusCreated},
	http.MethodPut:    {http.StatusOK},
	http.MethodDelete: {http.StatusOK, http.StatusNoContent},
}

type Client struct {
	*resty.Client
	Config  *Config
	baseURL string
	headers map[string]string
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
	// Format path variables
	path = formatPath(path, pathVars)

	req := c.R().SetContext(ctx)

	// Set global headers
	for k, v := range c.headers {
		req.SetHeader(k, v)
	}

	// Set custom headers
	for k, v := range headers {
		req.SetHeader(k, v)
	}

	// Set query params
	if len(params) > 0 {
		req.SetQueryParams(params)
	}

	// Set body nếu có
	if body != nil {
		req.SetHeader("Content-Type", "application/json")
		req.SetBody(body)
	}

	var resp *resty.Response
	var err error

	switch method {
	case http.MethodGet:
		resp, err = req.Get(path)
	case http.MethodPost:
		resp, err = req.Post(path)
	case http.MethodPut:
		resp, err = req.Put(path)
	case http.MethodDelete:
		resp, err = req.Delete(path)
	default:
		return errors.New("unsupported HTTP method: " + method)
	}
	if err != nil {
		return err
	}

	// Check status
	if !isValidStatus(method, resp.StatusCode()) {
		if c.Config.Debug {
			fmt.Printf("Request failed. Method: %s, URL: %s, Status: %d, Body: %s\n",
				method, path, resp.StatusCode(), string(resp.Body()))
		}
		return errors.New("request failed with status: " + resp.Status())
	}

	if result != nil {
		return json.Unmarshal(resp.Body(), result)
	}
	return nil
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
