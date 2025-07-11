# Go-Feign

`Go-Feign` là một thư viện Go lấy cảm hứng từ Feign của Java, giúp tạo các client REST API một cách khai báo (declarative) và dễ dàng. Bạn chỉ cần định nghĩa một struct với các trường là function, thêm các annotation và `Go-Feign` sẽ tự động tạo ra cài đặt cho bạn.

## Tính năng

- Client REST khai báo, dễ đọc và dễ bảo trì.
- Cấu hình bằng annotation ngay trên phương thức (`@GET`, `@POST`, `@Path`, `@Query`, ...).
- Tự động phân giải URL từ giá trị cấu hình (sử dụng Viper) hoặc giá trị cứng.
- Tự động serialize/deserialize JSON.
- Truyền `context.Context` một cách liền mạch.
- Hỗ trợ header động và tĩnh.

## Cách sử dụng

### 1. Định nghĩa Client Interface

Tạo một `struct` với các trường là các hàm. Sử dụng `feign` tag để định nghĩa các chi tiết của request HTTP.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"go-feign/feign" // Giả sử project của bạn nằm trong GOPATH
)

// Model ví dụ
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Job  string `json:"job"`
}

// Định nghĩa client
type UserClient struct {
	// Field trống để khai báo URL cơ sở cho client.
	// Có thể là URL cứng hoặc key trong file config (Viper).
	_ struct{} `feign:"@Url https://reqres.in/api"`

	// GET /users/{id}
	// @Path ánh xạ tham số `id` vào placeholder `{id}` trên URL.
	GetUserByID func(ctx context.Context, id int) (*User, error) `feign:"@GET /users/{id} | @Path id"`

	// POST /users
	// @Body chỉ định tham số `user` sẽ được serialize thành JSON body.
	CreateUser func(ctx context.Context, user User) (*User, error) `feign:"@POST /users | @Body user"`

	// GET /users?name={name}
	// @Query ánh xạ tham số `name` vào query parameter.
	FindUsersByName func(ctx context.Context, name string) ([]User, error) `feign:"@GET /users | @Query name"`

	// GET /search
	// @Queries ánh xạ một map thành các query parameters.
	Search func(ctx context.Context, queries map[string]string) (any, error) `feign:"@GET /search | @Queries queries"`

	// GET /with-headers
	// @Header và @Headers để thêm header vào request.
	GetWithHeaders func(ctx context.Context, token string, customHeaders map[string]string) (any, error) `feign:"@GET /with-headers | @Header Authorization | @Headers customHeaders"`
}
```

### 2. Khởi tạo và sử dụng Client

Sử dụng `feign.NewClient()` để tạo một client factory và sau đó gọi `Create()` để khởi tạo client của bạn.

```go
func main() {
	// 1. Tạo một feign client mới
	feignClient := feign.NewClient()

	// 2. Khởi tạo UserClient
	var userClient UserClient
	feignClient.Create(&userClient)

	// 3. Sử dụng các hàm trong client
	ctx := context.Background()

	// Lấy user có ID 2
	user, err := userClient.GetUserByID(ctx, 2)
	if err != nil {
		if httpErr, ok := err.(*feign.HttpError); ok && httpErr.StatusCode == 404 {
			fmt.Println("User not found, which is expected for this example.")
		} else {
			log.Fatalf("Lỗi khi lấy user: %v", err)
		}
	} else {
		fmt.Printf("✅ Lấy user thành công: %+v\n", user)
	}

	// Tạo user mới
	newUser := User{Name: "Morpheus", Job: "Leader"}
	createdUser, err := userClient.CreateUser(ctx, newUser)
	if err != nil {
		log.Fatalf("Lỗi khi tạo user: %v", err)
	}
	fmt.Printf("✅ Tạo user thành công: %+v\n", createdUser)
}

```

### 3. Cấu hình

#### URL cơ sở (Base URL)

Bạn có thể cấu hình URL cơ sở theo hai cách:

1.  **Qua `feign.NewClient(baseUrl)`**:
    ```go
    client := feign.NewClient("http://api.example.com")
    ```
2.  **Qua annotation `@Url`**:
    -   **URL cứng**:
        ```go
        type MyClient struct {
            _ struct{} `feign:"@Url http://api.example.com"`
            // ...
        }
        ```
    -   **Qua Viper**: Nếu giá trị không phải là URL, nó sẽ được xem là key để tra cứu trong Viper.
        ```go
        // config.yaml
        // my_api:
        //   url: http://api.example.com
        
        type MyClient struct {
            _ struct{} `feign:"@Url my_api.url"`
            // ...
        }
        ```

Annotation `@Url` sẽ ghi đè lên giá trị được truyền vào `NewClient`.

#### Cấu hình qua file YAML (với Viper)

`go-feign` được thiết kế để tích hợp tốt với [Viper](https://github.com/spf13/viper) cho việc quản lý cấu hình. Bạn có thể định nghĩa các thiết lập cho client trong một file `config.yaml`.

**Ví dụ `config.yaml`:**
```yaml
my_service:
  url: https://api.my-service.com/v1
  timeout: 15s
  retry_count: 3
  retry_wait: 2s
  debug: true
  headers:
    X-Api-Key: "your-secret-key"
    Content-Type: "application/json"
```

**Sử dụng trong code:**

Bạn có thể dùng hàm `feign.NewConfig("my_service")` để tải cấu hình này.

```go
import (
    "github.com/spf13/viper"
    "go-feign/feign"
    "log"
)

func setupViper() {
    viper.SetConfigName("config") // Tên file không có extension
    viper.AddConfigPath(".")      // Đường dẫn tìm file config
    viper.SetConfigType("yaml")
    if err := viper.ReadInConfig(); err != nil {
        log.Fatalf("Lỗi đọc file config: %s", err)
    }
}

func main() {
    setupViper()

    // Tải cấu hình với prefix "my_service"
    config := feign.NewConfig("my_service")
    client := feign.New(config)

    // ... sử dụng client
}
```

## Annotations

Các annotation được định nghĩa trong `feign` tag của mỗi trường, phân tách bởi `|`.

| Annotation   | Mô tả                                                                                      | Ví dụ                                            |
|--------------|--------------------------------------------------------------------------------------------|--------------------------------------------------|
| `@GET`       | Định nghĩa phương thức GET và đường dẫn.                                                   | `@GET /users/{id}`                               |
| `@POST`      | Định nghĩa phương thức POST và đường dẫn.                                                  | `@POST /users`                                   |
| `@PUT`       | Định nghĩa phương thức PUT và đường dẫn.                                                   | `@PUT /users/{id}`                               |
| `@DELETE`    | Định nghĩa phương thức DELETE và đường dẫn.                                                | `@DELETE /users/{id}`                            |
| `@Path`      | Ánh xạ một tham số của hàm vào một biến trên đường dẫn. Tên biến phải trùng khớp.             | `@Path userId`                                   |
| `@Body`      | Chỉ định một tham số làm body của request. Sẽ được mã hóa thành JSON.                      | `@Body user`                                     |
| `@Query`     | Ánh xạ một tham số của hàm vào một query parameter.                                        | `@Query page`                                    |
| `@Queries`   | Ánh xạ một tham số `map[string]string` thành nhiều query parameters.                         | `@Queries params`                                |
| `@Header`    | Ánh xạ một tham số của hàm vào một HTTP header.                                            | `@Header Authorization`                          |
| `@Headers`   | Ánh xạ một tham số `map[string]string` thành nhiều HTTP headers.                            | `@Headers customHeaders`                         |
| `@Url`       | (Dùng cho field struct rỗng) Định nghĩa URL cơ sở cho tất cả các request trong client đó.      | `@Url http://my-api.com`                         |

## Xử lý lỗi

Khi một request HTTP trả về mã trạng thái không thành công (không nằm trong khoảng 200-299), hàm sẽ trả về một lỗi kiểu `*feign.HttpError`. Bạn có thể kiểm tra lỗi này để lấy thông tin chi tiết.

```go
user, err := userClient.GetUserByID(ctx, 999) // Giả sử user 999 không tồn tại
if err != nil {
    if httpErr, ok := err.(*feign.HttpError); ok {
        fmt.Printf("Lỗi HTTP: %d %s\n", httpErr.StatusCode, httpErr.Status)
        fmt.Printf("Body: %s\n", httpErr.Body)
        // Output có thể là:
        // Lỗi HTTP: 404 Not Found
        // Body: {}
    } else {
        fmt.Printf("Lỗi không xác định: %v\n", err)
    }
}
```

## Sử dụng Client một cách tường minh (Programmatic Usage)

Ngoài cách tiếp cận khai báo ở trên, bạn cũng có thể sử dụng `feign.Client` một cách trực tiếp để thực hiện các lời gọi API. Điều này hữu ích khi bạn cần nhiều quyền kiểm soát hơn hoặc không muốn định nghĩa một struct client đầy đủ.

### 1. Khởi tạo Client

Đầu tiên, bạn cần tạo một instance của `feign.Config` và sau đó là `feign.Client`.

```go
import (
	"go-feign/feign"
	"time"
)

// Khởi tạo cấu hình
config := &feign.Config{
    Url:        "https://reqres.in/api",
    Timeout:    30 * time.Second,
    RetryCount: 3,
    RetryWait:  5 * time.Second,
    Headers: map[string]string{
        "Accept": "application/json",
    },
    Debug:      true,
}

// Hoặc bạn có thể dùng helper để đọc từ Viper
// config := feign.NewConfig("my_api_service") // Đọc config với prefix "my_api_service"

// Tạo client
client := feign.New(config)
```

### 2. Gọi API REST (CallREST)

`CallREST` là hàm linh hoạt nhất để thực hiện các lời gọi REST.

```go
type SingleUserResponse struct {
	Data User `json:"data"`
}

var result SingleUserResponse
pathVars := map[string]string{"userId": "2"}

err := client.CallREST(context.Background(), "GET", "/users/{userId}", pathVars, nil, nil, nil, &result)
if err != nil {
    log.Fatalf("Lỗi khi gọi CallREST: %v", err)
}

fmt.Printf("✅ Lấy user (CallREST) thành công: %+v\n", result.Data)
```

### 3. Gọi API SOAP (CallSOAP)

Thư viện cũng hỗ trợ thực hiện các lời gọi SOAP cơ bản.

```go
soapBody := `<soapenv:Envelope ...>...</soapenv:Envelope>`
var soapResult MySoapResult

err := client.CallSOAP(context.Background(), "/soap-endpoint", "MySoapAction", soapBody, &soapResult)
if err != nil {
    log.Fatalf("Lỗi khi gọi CallSOAP: %v", err)
}

fmt.Println("✅ Gọi SOAP thành công!")
```

### 4. Tải xuống file (Download)

Sử dụng hàm `Download` để tải file và ghi trực tiếp vào một `io.Writer`.

```go
import (
    "os"
)

file, err := os.Create("downloaded_file.zip")
if err != nil {
    log.Fatalf("Không thể tạo file: %v", err)
}
defer file.Close()

err = client.Download(context.Background(), "/files/archive.zip", file)
if err != nil {
    log.Fatalf("Lỗi khi tải file: %v", err)
}

fmt.Println("✅ Tải file thành công!")
```

## Mở rộng

`go-feign` có thể được mở rộng với các tính năng nâng cao trong tương lai:

- **Interceptors**: Thêm các interceptor để xử lý request và response (ví dụ: logging, thêm/xóa header, refresh token).
- **Custom Encoders/Decoders**: Hỗ trợ các định dạng dữ liệu khác ngoài JSON, ví dụ như XML, Protobuf.
- **Cơ chế Retry nâng cao**: Tích hợp các chính sách retry phức tạp hơn như exponential backoff.

## Đóng góp
PR, issue, góp ý đều rất hoan nghênh!

---
**Made with ❤️ by xhkzeroone**
