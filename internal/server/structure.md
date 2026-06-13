# Server的结构

## app.go
### 定义了App结构体
```go
type App struct {
    httpServer   *http.Server   // HTTP服务器实例
    pool         *pgxpool.Pool  // PostgreSQL连接池
    repos        *Repos         // 存储库实例，包含各种数据访问对象
    webDistDir   string         // 前端资源目录路径
}
```
### 关于app.httpServer的初始化
```go
app.httpServer = &http.Server{
    Addr:              fmt.Sprintf(":%s", cfg.Port),
    Handler:           NewRouter(app),
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       10 * time.Second,
    WriteTimeout:      15 * time.Second,
    IdleTimeout:       60 * time.Second,
}
```
Handler字段被设置为NewRouter(app)，这意味着HTTP服务器将使用NewRouter函数返回的路由器来处理传入的HTTP请求。

## router.go

### http.Handler接口
在Go语言中，http.Handler是一个接口，定义了一个ServeHTTP方法。任何实现了这个接口的类型都可以用来处理HTTP请求。  
ServeHTTP接收一个http.ResponseWriter和一个http.Request作为参数，http.ResponseWriter用于构建HTTP响应，而http.Request包含了客户端发送的HTTP请求的信息。他的作用是处理HTTP请求并生成相应的HTTP响应。  
NewRouter函数返回一个实现了http.Handler接口的路由器实例，这个实例会根据请求的URL路径和HTTP方法来分发请求到相应的处理函数。  

### ServeMux
ServeMux是Go标准库中提供的一个HTTP请求多路复用器（HTTP request multiplexer）。它实现了http.Handler接口，可以将不同的URL路径映射到不同的处理函数。当HTTP服务器接收到一个请求时，ServeMux会根据请求的URL路径来查找对应的处理函数，并调用它来处理请求。