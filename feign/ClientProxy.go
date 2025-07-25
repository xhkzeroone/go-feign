package feign

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/viper"
)

// HttpError giúp phân biệt lỗi HTTP như 401, 404, 500
type HttpError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HttpError) Error() string {
	return fmt.Sprintf("HTTP %d: %s - %s", e.StatusCode, e.Status, e.Body)
}

// Nếu value bắt đầu bằng http/https thì dùng luôn, ngược lại tra từ Viper
func resolveUrl(value string) string {
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return viper.GetString(value) // nếu không có thì trả về ""
}

// Create gán các hàm vào struct target (ví dụ: *UserClient)
func (c *Client) Create(target any) {
	t := reflect.TypeOf(target).Elem()
	v := reflect.ValueOf(target).Elem()

	baseUrl := extractBaseURLFromStruct(t, c.baseURL)
	c.SetBaseURL(baseUrl)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.Type.Kind() != reflect.Func {
			continue
		}

		methodType := field.Type
		validateFeignMethod(field, methodType)

		meta := parseTagInfo(field)
		fn := c.generateFuncHandler(methodType, meta, baseUrl)
		v.Field(i).Set(fn)
	}
}

func (c *Client) generateFuncHandler(methodType reflect.Type, meta tagMeta, baseUrl string) reflect.Value {
	return reflect.MakeFunc(methodType, func(args []reflect.Value) []reflect.Value {
		ctx := args[0].Interface().(context.Context)
		var body interface{}

		if len(meta.BodyParam) > 0 {
			for k := range meta.BodyParam {
				body = args[k].Interface()
				break // Chỉ hỗ trợ một body duy nhất
			}
		}

		// Replace path params
		pathProcessed := meta.Path
		for index, p := range meta.PathVars {
			placeholder := fmt.Sprintf("{%s}", p)
			pathProcessed = strings.ReplaceAll(pathProcessed, placeholder, fmt.Sprintf("%v", args[index].Interface()))
		}

		// Query
		queryParams := make(map[string]string)
		for k, v := range meta.Queries {
			queryParams[v] = fmt.Sprintf("%v", args[k].Interface())
		}
		for k, _ := range meta.MapQueries {
			if m, ok := args[k].Interface().(map[string]string); ok {
				for k2, v2 := range m {
					queryParams[k2] = v2
				}
			}
		}

		// Headers
		headersMap := make(map[string]string)
		for index, h := range meta.Headers {
			headersMap[h] = fmt.Sprintf("%v", args[index].Interface())
		}
		for k, _ := range meta.MapHeaders {
			if m, ok := args[k].Interface().(map[string]string); ok {
				for k2, v2 := range m {
					headersMap[k2] = v2
				}
			}
		}

		// Chuẩn hóa request cho middleware
		req := &Request{
			Context:  ctx,
			Method:   meta.HttpMethod,
			Path:     pathProcessed,
			PathVars: map[string]string{}, // Đã xử lý path rồi
			Params:   queryParams,
			Headers:  headersMap,
			Body:     body,
			Result:   nil, // Sẽ gán sau
		}

		retType := methodType.Out(0)
		isPointer := retType.Kind() == reflect.Pointer
		var out reflect.Value
		if isPointer {
			out = reflect.New(retType.Elem())
		} else {
			out = reflect.New(retType)
		}
		req.Result = out.Interface()

		handler := func(r *Request) error {
			rResty := c.R().SetContext(r.Context)
			for k, v := range c.headers {
				rResty.SetHeader(k, v)
			}
			for k, v := range r.Headers {
				rResty.SetHeader(k, v)
			}
			if len(r.Params) > 0 {
				rResty.SetQueryParams(r.Params)
			}
			if r.Body != nil && r.Method != "GET" {
				rResty.SetHeader("Content-Type", "application/json")
				rResty.SetBody(r.Body)
			}
			fmt.Printf("➡️ %s: %s\n", r.Method, baseUrl+r.Path)
			resp, err := rResty.Execute(r.Method, r.Path)
			if err != nil {
				return &HttpError{Status: "connection failed", Body: err.Error()}
			}
			if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
				return &HttpError{StatusCode: resp.StatusCode(), Status: resp.Status(), Body: string(resp.Body())}
			}
			if err := json.Unmarshal(resp.Body(), r.Result); err != nil {
				fmt.Println("❌ JSON Decode Error:", err)
				return fmt.Errorf("unmarshal failed: %w", err)
			}
			return nil
		}

		var err error
		if len(c.middlewares) > 0 {
			err = c.buildChain(handler)(req)
		} else {
			err = handler(req)
		}

		if err != nil {
			return []reflect.Value{reflect.Zero(retType), reflect.ValueOf(err)}
		}
		if isPointer {
			return []reflect.Value{out, reflect.Zero(methodType.Out(1))}
		}
		return []reflect.Value{out.Elem(), reflect.Zero(methodType.Out(1))}
	})
}

type tagMeta struct {
	HttpMethod string
	Path       string
	BodyParam  map[int]string
	PathVars   map[int]string
	Headers    map[int]string
	Queries    map[int]string
	MapHeaders map[int]string
	MapQueries map[int]string
}

func parseTagInfo(method reflect.StructField) tagMeta {
	doc := method.Tag.Get("feign")
	methodType := method.Type

	meta := tagMeta{
		BodyParam:  make(map[int]string),
		PathVars:   make(map[int]string),
		Headers:    make(map[int]string),
		Queries:    make(map[int]string),
		MapHeaders: make(map[int]string),
		MapQueries: make(map[int]string),
	}

	for j, line := range strings.Split(doc, "|") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		tag := strings.TrimPrefix(parts[0], "@")
		value := parts[1]

		switch strings.ToUpper(tag) {
		case "GET", "POST", "PUT", "DELETE":
			meta.HttpMethod = strings.ToUpper(tag)
			meta.Path = value
		case "PATH":
			meta.PathVars[j] = value
		case "HEADER":
			meta.Headers[j] = value
		case "BODY":
			meta.BodyParam[j] = value
		case "QUERY":
			meta.Queries[j] = value
		case "HEADERS":
			inType := methodType.In(j)
			if inType.Kind() == reflect.Map && inType.Key().Kind() == reflect.String && inType.Elem().Kind() == reflect.String {
				meta.MapHeaders[j] = value
			}
		case "QUERIES":
			inType := methodType.In(j)
			if inType.Kind() == reflect.Map && inType.Key().Kind() == reflect.String && inType.Elem().Kind() == reflect.String {
				meta.MapQueries[j] = value
			}
		}
	}
	return meta
}

func extractBaseURLFromStruct(t reflect.Type, defaultURL string) string {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type == reflect.TypeOf(struct{}{}) {
			tag := field.Tag.Get("feign")
			for _, line := range strings.Split(tag, "|") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "@Url") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						if url := resolveUrl(parts[1]); url != "" {
							return url
						}
					}
				}
			}
		}
	}
	return defaultURL
}

func validateFeignMethod(field reflect.StructField, methodType reflect.Type) {
	if methodType.NumIn() < 1 {
		panic(fmt.Sprintf("method %s must have at least one parameter (context.Context)", field.Name))
	}
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	if !methodType.In(0).Implements(ctxType) {
		panic(fmt.Sprintf("method %s first parameter must be context.Context", field.Name))
	}
	if methodType.NumOut() != 2 || !methodType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		panic(fmt.Sprintf("method %s must return (*T, error)", field.Name))
	}
}
