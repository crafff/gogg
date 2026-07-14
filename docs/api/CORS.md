# 跨域访问（CORS）

## 基本概念

### 前端页面

前端一般是运行在浏览器中的一个网页（HTML）。网页本身没有"自己的地址"，但它是通过一个 URL 下载到浏览器中的，因此浏览器知道它来自哪个地址。

例如：

```
访问：https://shop.example.com/index.html
```

浏览器会：

1. 下载 `index.html`
2. 解析 HTML
3. 执行其中的 JavaScript

因此，这段 JavaScript 就属于这个网页（Document）。

---

### 前端发送请求

网页中的 JavaScript 可以向其他服务器发送 HTTP 请求，例如：

```javascript
fetch("https://api.example.com/data")
```

---

### Origin（源）

Origin 用来唯一标识一个网页或资源，由三部分组成：

```
Origin = scheme + host + port
```

例如：

```
前端页面：
https://shop.example.com/index.html

Origin：
https://shop.example.com
```

请求目标：

```
https://api.example.com/data
```

请求目标（API）的 Origin：

```
https://api.example.com
```

> **注意**
>
> 严格来说，Origin 是 URL 的属性，而不是"前端"或"后端"拥有的属性。为了方便讨论，人们通常会说"前端 Origin"和"后端 Origin"。

---

### 什么是跨域（Cross-Origin）

当网页所在 Origin 与请求目标的 Origin 不一致时，就称为跨域访问。

只要 **协议（scheme）**、**主机（host）**、**端口（port）** 三者任意一个不同，就属于跨域。

| 页面地址 | 请求地址 | 是否跨域 |
|----------|----------|----------|
| https://a.com | https://a.com | ❌ 不跨域 |
| https://a.com | https://api.a.com | ✅ Host 不同 |
| https://a.com | http://a.com | ✅ 协议不同 |
| https://a.com | https://a.com:8080 | ✅ 端口不同 |

---

## 浏览器的同源策略（Same-Origin Policy）

浏览器为了安全，引入了 **同源策略（Same-Origin Policy）**。

需要注意的是：

**同源策略并不是默认禁止发送跨域请求。**

真正限制的是：

> **浏览器默认不允许 JavaScript 读取未经授权的跨域响应。**

例如：

```javascript
fetch("https://api.example.com/data")
```

对于简单请求：

```
浏览器
    │
    ├── 发送 GET 请求
    │
服务器收到请求
    │
服务器返回响应
    │
浏览器检查 CORS
    │
    ├── 允许 → JS 可以读取响应
    └── 不允许 → JS 无法读取响应
```

服务器实际上已经收到了请求，也已经返回了响应，只是浏览器不会把响应交给 JavaScript。

只有对于需要 **预检（Preflight）** 的请求，如果预检失败，浏览器才不会发送真正的请求。

---

## 非浏览器环境

浏览器才会执行同源策略和 CORS 检查。

例如：

- Node.js
- curl
- Postman
- Python requests
- Go HTTP Client

这些客户端都可以直接发送 HTTP 请求，不受浏览器 CORS 的限制。

因此：

```
浏览器：
    受 CORS 限制

Node.js：
    不受 CORS 限制

curl：
    不受 CORS 限制
```

---

## 请求头中的 Origin

浏览器在发送跨域请求时，会自动添加：

```
Origin: https://shop.example.com
```

例如：

```
GET /data HTTP/1.1

Origin: https://shop.example.com
```

浏览器根据当前网页（Document）的 Origin 自动填写该 Header。

JavaScript **不能修改** `Origin`。

例如：

```javascript
fetch(url, {
    headers: {
        Origin: "https://google.com"
    }
})
```

浏览器会忽略或直接报错，因为 `Origin` 属于浏览器保护的请求头（Forbidden Request Header）。

但是，在非浏览器环境中可以随意伪造：

```bash
curl \
-H "Origin: https://google.com" \
https://api.example.com/data
```

因此：

> **Origin 不能作为身份认证依据。**

---

## CORS 的作用

CORS（Cross-Origin Resource Sharing）的核心目的不是阻止请求，而是：

> **防止恶意网页读取未经授权的跨域响应。**

例如：

```
evil.com
    │
    ├── fetch(bank.com)
    │
浏览器发送请求
    │
银行服务器返回数据
    │
浏览器检查 CORS
    │
    ├── 允许 → JS 可读取数据
    └── 不允许 → JS 无法读取数据
```

请求已经发送成功。

真正阻止的是：

**JavaScript 读取响应。**

---

## CORS 与身份认证

CORS 并不能替代身份认证。

后端仍然需要：

- Session
- JWT
- OAuth
- API Key

等身份认证机制。

原因是：

任何人都可以使用：

- curl
- Node.js
- Python
- Go

伪造请求。

因此：

```
CORS：
    控制浏览器是否允许读取响应

身份认证：
    判断请求者是否有权限访问资源
```

二者解决的是完全不同的问题。

---

# 跨域访问流程

## 一、简单请求（Simple Request）

### 满足以下条件才属于简单请求

### 1）请求方法

仅允许：

- GET
- POST
- HEAD

---

### 2）请求头

只能包含浏览器规定的 CORS Safelisted Headers，例如：

- Accept
- Accept-Language
- Content-Language

Content-Type 仅允许：

- application/x-www-form-urlencoded
- multipart/form-data
- text/plain

---

### 3）没有其他需要预检的请求头

例如：

```
Authorization
```

或

```
X-Token
```

都会导致预检。

---

## 简单请求流程

### 1）浏览器直接发送请求

例如：

```
GET /data
Origin: https://shop.example.com
```

不会先发送 OPTIONS。

---

### 2）服务器返回响应

如果允许跨域，需要返回：

```
Access-Control-Allow-Origin
```

例如：

```
Access-Control-Allow-Origin:
https://shop.example.com
```

也可以：

```
*
```

> **注意：**
>
> 当允许携带 Cookie（`Access-Control-Allow-Credentials: true`）时，`Access-Control-Allow-Origin` 不能设置为 `*`。

---

### 3）浏览器检查响应头

如果：

```
Access-Control-Allow-Origin
```

与当前网页 Origin 匹配：

```
JavaScript 可以读取响应
```

否则：

```
浏览器阻止 JavaScript 读取响应
```

服务器实际上已经处理了请求。

---

# 二、预检请求（Preflight Request）

浏览器在认为请求可能产生副作用时，会先发送 OPTIONS 请求。

常见触发条件：

## 1）请求方法不是

- GET
- POST
- HEAD

例如：

- PUT
- DELETE
- PATCH

---

## 2）使用了非 Safelisted Header

例如：

```
Authorization

X-Token

X-Request-ID
```

等等。

---

## 3）Content-Type 不是三种安全值

例如：

```
application/json
```

即使是 POST，也会触发预检。

---

## 预检流程

### 1）浏览器发送 OPTIONS 请求

例如：

```
OPTIONS /data

Origin:
https://shop.example.com

Access-Control-Request-Method:
PUT

Access-Control-Request-Headers:
Authorization
```

---

### 2）服务器返回

例如：

```
Access-Control-Allow-Origin

Access-Control-Allow-Methods

Access-Control-Allow-Headers

Access-Control-Max-Age
```

HTTP 状态码通常是：

```
200
```

或

```
204
```

真正重要的是这些 Header，而不是状态码本身。

---

### 3）浏览器检查

浏览器检查：

- Origin 是否允许
- Method 是否允许
- Header 是否允许

全部通过：

```
发送真正请求
```

否则：

```
浏览器不会发送真正请求
```

---

### 4）发送真正请求

浏览器发送：

```
PUT /data
```

---

### 5）服务器返回真正响应

服务器仍然需要再次返回：

```
Access-Control-Allow-Origin
```

否则浏览器仍然不会把响应交给 JavaScript。

---

# Go CORS 中间件示例

```go
// CORS rejects unknown origins instead of echoing back the legacy
// permissive "*". `allowed` is a slice of exact origins (scheme + host + port);
// an empty slice means CORS is disabled (same-origin only).
func CORS(allowed []string) func(http.Handler) http.Handler {
    allow := make(map[string]struct{}, len(allowed))
    for _, o := range allowed {
        allow[strings.TrimSpace(o)] = struct{}{}
    }

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

            origin := r.Header.Get("Origin")

            if origin != "" {
                if _, ok := allow[origin]; ok {
                    w.Header().Set("Access-Control-Allow-Origin", origin)
                    w.Header().Set("Vary", "Origin")
                    w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
                    w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, "+HeaderRequestID)
                    w.Header().Set("Access-Control-Max-Age", "600")
                }
            }

            if r.Method == http.MethodOptions {
                w.WriteHeader(http.StatusNoContent)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

---

## 代码说明

### allowed

允许跨域访问的 Origin 白名单。

例如：

```go
allowed := []string{
    "https://shop.example.com",
}
```

---

### Origin

从请求头读取：

```
Origin
```

判断是否属于允许访问的网站。

---

### Access-Control-Allow-Origin

告诉浏览器：

哪些 Origin 可以读取响应。

---

### Vary: Origin

告诉浏览器、CDN、反向代理：

响应内容会随着 `Origin` 不同而变化。

避免缓存错误地把 A 网站的响应返回给 B 网站。

---

### Access-Control-Allow-Methods

允许浏览器发送哪些 HTTP 方法。

例如：

```
GET
POST
OPTIONS
```

---

### Access-Control-Allow-Headers

允许浏览器携带哪些请求头。

例如：

```
Authorization
Content-Type
```

---

### Access-Control-Max-Age

表示浏览器可以缓存 **预检（OPTIONS）结果** 的时间。

例如：

```
600 秒
```

在缓存有效期内，浏览器不会再次发送 OPTIONS。

注意：

缓存的是 **预检结果**，不是实际请求的响应。

---

### OPTIONS

代码中：

```go
if r.Method == http.MethodOptions {
    w.WriteHeader(http.StatusNoContent)
    return
}
```

表示所有 OPTIONS 请求都会直接返回 204。

严格来说，这里并没有进一步判断它是否是一个真正的 CORS 预检请求（例如是否包含 `Origin` 和 `Access-Control-Request-Method`），不过对于大多数 Web API 来说，这种实现已经足够简单且常见。

## 总结
一句话总结：

- 同源策略（Same-Origin Policy）：浏览器的安全策略，限制跨源资源访问。
- CORS：同源策略的一种放宽机制，由服务器通过响应头告诉浏览器哪些源可以读取响应。
- 简单请求：直接发送请求，再检查响应是否允许读取。
- 预检请求（OPTIONS）：先询问服务器是否允许，允许后才发送真正请求。
- CORS 只约束浏览器，不约束 curl、Node.js、Postman 等客户端，也不能替代身份认证。