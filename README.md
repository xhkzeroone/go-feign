# Go-Feign

**Go-Feign** là thư viện Go lấy cảm hứng từ Feign (Java), giúp bạn tạo REST/SOAP client dạng khai báo, dễ mở rộng, hỗ trợ middleware chain, metrics, retry, cấu hình động qua YAML/Viper.

## Tính năng nổi bật

- Khai báo client qua struct và tag, mapping API tự động.
- Hỗ trợ các annotation/tag: `@GET`, `@POST`, `@PUT`, `@DELETE`, `@Path`, `@Query`, `@Header`, `@Body`, `@Headers`, `@Queries`.
- Middleware chain: logging, metrics, retry, custom logic.
- Cấu hình động qua YAML/Viper, dễ tích hợp CI/CD.
- Hỗ trợ REST, SOAP, download file, custom error (`HttpError`).

---

## Cài đặt

```bash
go get github.com/xhkzeroone/go-feign
```

---

## Cấu hình

Bạn có thể cấu hình qua YAML hoặc code:

**resources/config.yml**
```yaml
feign:
  account:
    url: http://localhost:8081/api/v1
    timeout: 40s
    retry_count: 3
    retry_wait: 1s
    debug: false
    headers:
      Token: Bearer 1231231231231231231231
```

**Tạo config trong code:**
```go
cfg := feign.NewConfig("feign.account")
client := feign.New(cfg)
```

---

## Khai báo & Khởi tạo Client

### 1. Function field + Create (giống Feign Java)

```go
type MyClient struct {
    _ struct{} `feign:"@Url http://api.example.com"`
    GetUser func(ctx context.Context, id string, token string) (*User, error) `feign:"@GET /users/{id} | @Path id | @Header Authorization"`
}
client := &MyClient{}
feignClient := feign.New(cfg)
feignClient.Create(client)
```

### 2. Embed feign.Client + Default (Go style)

```go
type UserClient struct {
    *feign.Client
}
func NewUserClient(c *feign.Client) *UserClient { return &UserClient{Client: c} }
userClient := feign.Default(cfg, NewUserClient)
```

---

## Annotation/tag hỗ trợ

- `@GET`, `@POST`, `@PUT`, `@DELETE`: Định nghĩa HTTP method và path.
- `@Path`: Tham số path.
- `@Query`: Tham số query string.
- `@Header`: Tham số header.
- `@Body`: Body request (POST/PUT).
- `@Headers`: Map[string]string cho nhiều header động.
- `@Queries`: Map[string]string cho nhiều query động.

**Ví dụ:**
```go
GetUser func(ctx context.Context, id string, token string) (*User, error) `feign:"@GET /users/{id} | @Path id | @Header Authorization"`
GetUserByIds func(ctx, user string, queries map[string]string, headers map[string]string, id string) (*User, error) `feign:"@GET /users/{user} | @Path user | @Queries queries | @Headers headers | @Query id"`
```

---

## Sử dụng Client

**Gọi REST:**
```go
user, err := client.GetUser(ctx, "123", "Bearer token")
```

**Gọi REST thủ công:**
```go
opt := feign.ReqOption{
    Context: ctx,
    Method: "GET",
    Path: "/users/123",
    Headers: map[string]string{"Authorization": "Bearer token"},
}
err := client.CallREST(opt, &result)
```

**Gọi SOAP:**
```go
err := client.CallSOAP(ctx, "/soap", "Action", body, &result)
```

**Download file:**
```go
err := client.Download(ctx, "/file", fileWriter)
```

**Xử lý lỗi:**
```go
if err, ok := err.(*feign.HttpError); ok {
    fmt.Println("Status:", err.StatusCode, "Body:", err.Body)
}
```

---

## Middleware nâng cao

Bạn có thể đăng ký nhiều middleware (logging, retry, metrics, ...):

```go
// Middleware log request
client.Use(func(next feign.Handler) feign.Handler {
    return func(req *feign.Request) error {
        fmt.Println("Before", req.Method, req.Path)
        err := next(req)
        fmt.Println("After", req.Method, req.Path)
        return err
    }
})

// Middleware thêm header tự động
client.Use(func(next feign.Handler) feign.Handler {
    return func(req *feign.Request) error {
        req.Headers["X-Request-ID"] = "my-request-id"
        req.Headers["X-Trace-Id"] = "trace-123"
        req.Headers["X-My-Custom"] = "custom-value"
        return next(req)
    }
})
```

**Ví dụ middleware retry và metrics:**
```go
client.Use(func(next feign.Handler) feign.Handler {
    return func(req *feign.Request) error {
        for i := 0; i < 3; i++ {
            err := next(req)
            if err == nil { return nil }
            time.Sleep(200 * time.Millisecond)
        }
        return fmt.Errorf("retry failed")
    }
})
client.Use(func(next feign.Handler) feign.Handler {
    return func(req *feign.Request) error {
        start := time.Now()
        err := next(req)
        fmt.Printf("Request took %v\n", time.Since(start))
        return err
    }
})
```

---

## Interceptor/Hook

- `OnBeforeRequest`: can thiệp trước khi gửi request (thêm header, log, ...).
- `OnAfterResponse`: xử lý sau khi nhận response.

```go
client.OnBeforeRequest(func(c *resty.Client, r *resty.Request) error {
    r.SetHeader("X-Request-ID", "some-id")
    return nil
})
client.OnAfterResponse(func(c *resty.Client, r *resty.Response) error {
    fmt.Println("Response status:", r.Status())
    return nil
})
```

---

## Đóng góp

PR, issue, góp ý đều rất hoan nghênh!

---

**Made with ❤️ by xhkzeroone**
