# HTTP

外部资源：

* https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/Messages

---

## HTTP 基本概念

HTTP（HyperText Transfer Protocol，超文本传输协议）是一种**应用层协议（Application Layer Protocol）**，用于规定客户端（Client）和服务器（Server）之间通信时消息（Message）的格式和交互规则。

HTTP 协议由 **IETF（Internet Engineering Task Force）** 通过一系列 **RFC（Request for Comments）** 标准定义和维护，目前最新的主流版本是 **HTTP/3**。

HTTP 主要规定了：

* 请求（Request）的格式
* 响应（Response）的格式
* HTTP 方法（Method）的语义
* HTTP 状态码（Status Code）的含义
* 常见 Header 的含义
* 客户端与服务器之间的通信规则

如果违反 HTTP 协议规范，客户端或服务器可能无法正确解析请求或响应，例如返回 **400 Bad Request**，或者直接关闭连接。

---

### 关于 HTTP 请求是否合法

浏览器本身就是 HTTP 协议的实现，因此浏览器几乎不会发送格式非法的 HTTP 请求。

浏览器会自动补充一些必要的 Header，例如：

* Host
* User-Agent
* Accept

JavaScript 也不能修改一些受保护的 Header，例如：

* Host
* Origin
* Content-Length

curl、Postman 等工具同样会自动生成符合 HTTP 规范的请求，但允许用户覆盖更多 Header，因此更容易构造一些特殊甚至不符合规范的请求。

如果直接使用 Socket（TCP）编程，则需要完全手动拼接 HTTP 报文。如果格式错误（例如请求行格式错误、缺少 Host 等），服务器通常会返回 **400 Bad Request** 或直接关闭连接。

---

# HTTP 报文（HTTP Message）

HTTP 通信本质上就是客户端和服务器之间交换 **HTTP Message**。

HTTP Message 分为：

* Request（请求）
* Response（响应）

---

## Request（请求）

格式如下：

```text
<method> <request-target> <version>
<headers>

<body>
```

例如：

```http
GET /api/v1/users HTTP/1.1
Host: example.com
User-Agent: curl/8.0
Accept: */*

<请求体（可选）>
```

---

### Request Line（请求行）

第一行叫做 **Request Line（请求行）**，包含：

* HTTP Method
* Request Target
* HTTP Version

例如：

```http
GET /api/v1/users HTTP/1.1
```

其中：

* `GET`：请求方法（Method）
* `/api/v1/users`：Request Target
* `HTTP/1.1`：HTTP 协议版本

严格来说，这里的第二部分叫 **Request Target**。

对于绝大多数 REST API，它通常就是 URL 的 Path。

例如：

完整 URL：

```text
https://example.com/api/v1/users
```

HTTP 请求：

```http
GET /api/v1/users HTTP/1.1
```

其中：

```text
/api/v1/users
```

就是 Request Target（通常也是 URL 的 Path）。

---

### Request Headers（请求头）

请求头是一组 **Key: Value** 键值对，用于携带客户端发送给服务器的附加信息。

例如：

```http
Host: example.com
User-Agent: curl/8.0
Accept: */*
```

---

### Blank Line（空行）

请求头结束后必须有一个空行。

它表示：

> Header 已结束，下面开始 Body。

---

### Request Body（请求体）

Request Body 是可选的。

通常：

* POST
* PUT
* PATCH

等请求会携带请求体。

---

## HTTP 请求在网络上传输的是什么？

HTTP/1.x 在网络上传输的是 **字节流（Byte Stream）**。

例如：

```http
GET /users HTTP/1.1
Host: example.com
```

真正发送到 TCP 中的是这些字符编码后的字节，例如：

```text
47 45 54 20 2F ...
```

（ASCII / UTF-8 编码后的字节）

HTTP 本身不是：

* JSON
* Base64
* XML

HTTP 只是规定：

这些字节应该按照什么格式排列。

服务器收到字节流后，由 HTTP Parser 按照 HTTP 协议解析出：

* Method
* Request Target
* Header
* Body

---

## Response（响应）

格式如下：

```text
<version> <status_code> <reason_phrase>
<headers>

<body>
```

例如：

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "id": 1,
  "name": "John Doe"
}
```

---

### Status Line（状态行）

响应第一行叫 **Status Line（状态行）**。

例如：

```http
HTTP/1.1 200 OK
```

包括：

* HTTP Version
* Status Code
* Reason Phrase

其中：

Reason Phrase（例如 `OK`）只是对状态码的文字说明，现在已经是可选的。

---

### HTTP 状态码分类

HTTP 状态码是三位数字。

常见分类：

* 1xx：信息（Informational）
* 2xx：成功（Success）
* 3xx：重定向（Redirection）
* 4xx：客户端错误（Client Error）
* 5xx：服务器错误（Server Error）

---

# HTTP 方法（Method）

HTTP 定义了多种请求方法，每种方法都有自己的语义。

1. **GET**

   请求指定资源。

   通常用于读取数据。

2. **POST**

   将请求体提交给目标资源处理。

   服务器决定如何处理这份数据。

   REST API 中通常用于创建资源。

3. **PUT**

   用请求体替换指定资源的当前状态。

   REST API 中通常用于整体更新资源，也可以用于创建客户端指定 URI 的资源。

4. **DELETE**

   删除指定资源。

5. **PATCH**

   对指定资源进行部分修改。

6. **HEAD**

   类似 GET，但只返回响应头，不返回响应体。

7. **OPTIONS**

   查询资源支持的通信选项。

   浏览器跨域请求时通常用于 CORS 预检。

8. **TRACE**

   回显服务器收到的请求。

   主要用于诊断。

9. **CONNECT**

   建立到目标服务器的隧道。

   常用于 HTTPS Proxy。

---

## POST 和 PUT 的区别

一句话总结：

* **POST**：把数据提交给目标资源，由服务器决定如何处理。
* **PUT**：用请求体替换指定资源的当前状态。

HTTP 标准中：

POST 更强调 **提交（Submit）**。

PUT 更强调 **替换（Replace）**。

REST API 中通常：

* POST 用于创建资源。
* PUT 用于整体更新资源。

另外：

PUT 是 **幂等（Idempotent）** 的。

多次发送相同的 PUT 请求，最终资源状态相同。

POST 不是幂等的，多次发送可能产生多个资源或重复副作用。

---

# HTTP 状态码

常见状态码：

* 200 OK
* 201 Created
* 204 No Content
* 400 Bad Request
* 401 Unauthorized
* 403 Forbidden
* 404 Not Found
* 500 Internal Server Error

---

# HTTP Header

HTTP Header 是 HTTP 报文中的元数据，本质上是一组 **Key: Value** 键值对。

例如：

```http
Host: example.com
Content-Type: application/json
```

---

## 常见请求头

* Host
* User-Agent
* Accept
* Content-Type
* Authorization

---

## 常见响应头

* Content-Type
* Content-Length

---

## Host

在 **HTTP/1.1** 中，Host 是必需的 Header。

它用于指定目标服务器的主机名（以及端口），从而支持虚拟主机（Virtual Host）。

---

# HTTP Body

HTTP Body 可以有，也可以没有。

例如：

* GET 通常没有请求体。
* POST 通常携带请求体。
* PUT 通常携带请求体。
* PATCH 通常携带请求体。
* DELETE **可以**携带请求体，但绝大多数 REST API 不使用请求体。
* 响应状态码为 **204 No Content** 时，响应体为空。

HTTP 协议本身并不规定 Body 的内容格式。

Body 可以是：

* JSON
* XML
* HTML
* PDF
* 图片
* 视频
* ZIP
* 任意二进制数据

具体格式由：

```http
Content-Type
```

决定。

例如：

* application/json
* application/x-www-form-urlencoded
* multipart/form-data
* text/plain

---

# HTTP/1.1 与 HTTP/2

HTTP/2 相比 HTTP/1.1 的主要改进：

1. **二进制分帧（Binary Framing）**

   HTTP/1.1 使用文本格式。

   HTTP/2 使用二进制帧。

2. **真正的多路复用（Multiplexing）**

   HTTP/1.1 支持 Keep-Alive，可以复用 TCP 连接。

   但同一个连接上的请求通常需要按顺序处理，存在应用层队头阻塞。

   HTTP/2 支持真正的多路复用，可以同时发送多个请求和响应。

3. **Header 压缩**

   HTTP/2 使用 HPACK 压缩 Header。

---

# HTTP/2 与 HTTP/3

HTTP/3 的主要变化：

1. **传输层**

   HTTP/2 基于 TCP。

   HTTP/3 基于 QUIC（运行在 UDP 之上）。

2. **连接建立更快**

   QUIC 减少了连接建立所需的 RTT。

3. **更好的丢包恢复**

   HTTP/3 消除了 TCP 层的队头阻塞问题，在丢包情况下性能更好。

4. **安全性**

   HTTP/2 标准允许明文（h2c）和 TLS 两种方式，但浏览器几乎都只支持 HTTPS。

   HTTP/3 始终运行在 **QUIC + TLS 1.3** 之上，因此默认使用加密传输。

---

# 总结

HTTP 是一种应用层协议，用于规定客户端和服务器之间消息（Message）的格式和通信规则。

HTTP Message 由：

* Request Line / Status Line
* Header
* Blank Line
* Body

组成。

其中：

* Header 是 HTTP 自己认识的数据，用于描述请求或响应。
* Body 是业务数据，HTTP 不关心其具体内容，只负责传输，具体格式由 `Content-Type` 决定。

HTTP/1.1 使用文本格式表示 HTTP Message。

HTTP/2 将 HTTP Message 编码成二进制帧。

HTTP/3 基于 QUIC，实现了更低延迟、更好的丢包恢复能力，并默认使用 TLS 1.3 加密。
