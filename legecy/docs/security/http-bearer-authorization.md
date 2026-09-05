# HTTP Bearer Authentication

外部资源：

* https://blog.postman.com/what-is-a-bearer-token/
* https://swagger.io/docs/specification/v3_0/authentication/bearer-authentication/

---

## 基本概念

HTTP 定义了一个专门用于携带身份凭据（Credentials）的请求头：

```text
Authorization: <scheme> <credentials>
```

其中：

* **Scheme**：认证方案（Authentication Scheme）
* **Credentials**：认证凭据（Credentials）

例如：

```http
Authorization: Basic YWxpY2U6MTIzNDU2
```

```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

服务器收到 `Authorization` 后，会首先读取 **Scheme**，再决定如何解析后面的 Credentials。

常见的 Authentication Scheme：

* Basic Authentication
* Bearer Authentication
* Digest Authentication

---

## Bearer Authentication

Bearer Authentication 是 HTTP 定义的一种 **Authentication Scheme**。

它定义于 **RFC 6750（OAuth 2.0 Bearer Token Usage）**。

虽然 RFC 是为 OAuth 2.0 制定的，但如今 Bearer Authentication 已广泛用于各种 REST API，并不仅限于 OAuth 2.0。

Bearer 的含义是：

> **持有者（Bearer）**。

也就是说：

> **谁持有（Bear）这个 Token，谁就可以使用它访问资源。**

因此：

Bearer Token 更像一张数字门禁卡。

服务器通常不会关心：

> "是谁拿到这张 Token 的。"

而只会验证：

> "这张 Token 是否合法。"

---

# Bearer Authentication 格式

Bearer Authentication 的格式如下：

```http
Authorization: Bearer <token>
```

例如：

```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

这里：

* Scheme：`Bearer`
* Credentials：`<token>`

需要注意的是：

Bearer RFC **只规定了认证格式**：

```text
Authorization: Bearer <token>
```

并**没有规定 Token 的具体格式**。

因此：

Bearer Token 可以是：

* JWT
* Opaque Token（随机字符串）
* UUID
* OAuth Access Token
* 其他自定义格式

很多现代 REST API 中，Bearer Token 通常采用 JWT 格式，但这并不是 RFC 的要求。

---

# Bearer 与 JWT 的关系

Bearer 和 JWT 并不是同一个概念。

它们属于两个不同的层次。

```text
Authorization Header
        │
        ▼
Bearer Authentication
        │
        ▼
Bearer Token
        │
        ├── JWT（最常见）
        │
        ├── Opaque Token
        │
        └── 其他 Token 格式
```

一句话总结：

> **Bearer 是 HTTP Authentication Scheme。**

> **JWT 是 Token 的一种格式（Format）。**

Bearer Token 可以是 JWT。

但 Bearer Token 并不一定是 JWT。

---

# Bearer Authentication 使用流程

典型流程如下：

1. Client 向认证服务（Authentication Server）或应用服务器登录。
2. 服务器验证用户身份。
3. 服务器生成 Access Token，并返回给 Client。
4. Client 在后续请求中，将 Token 放入 HTTP Header：

```http
Authorization: Bearer <token>
```

5. 服务器验证 Token：

* 合法：返回资源。
* 非法或过期：返回 **401 Unauthorized**。

6. Access Token 过期后：

* 如果客户端仍然持有合法的 Refresh Token，可以使用 Refresh Token 获取新的 Access Token；
* 否则，需要重新登录。

---

## 登录获取 Token

例如：

```http
POST /api/auth/login HTTP/1.1
Host: api.example.com
Content-Type: application/json

{
  "username": "developer@example.com",
  "password": "secure_password"
}
```

服务器返回：

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

---

## 使用 Token 访问资源

```http
GET /api/users/profile HTTP/1.1
Host: api.example.com
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
Accept: application/json
```

服务器验证 Token：

* 合法 → 返回资源。
* 非法 → 返回 `401 Unauthorized`。

---

# Token 的分类

Token 可以从两个不同的维度进行分类：

* 按**格式（Format）**
* 按**用途（Purpose）**

不要混淆这两个概念。

---

## 按格式（Format）

### 1. Opaque Token

Opaque Token（不透明 Token）本质上就是一串随机字符串。

例如：

```text
A83F91BCDD0F...
```

服务器收到后，需要查询数据库或 Redis 才能知道它对应哪个用户。

特点：

* 无法从 Token 本身获取信息
* 可以立即失效
* 方便撤销（Revoke）

---

### 2. JWT（JSON Web Token）

JWT 是一种结构化 Token。

格式：

```text
xxxxx.yyyyy.zzzzz
```

包含：

* Header
* Payload（Claims）
* Signature

例如：

```text
eyJhbGciOiJIUzI1NiIs...

.

eyJzdWIiOiIxMjMiLCJyb2xlIjoiYWRtaW4ifQ

.

AbCdEfGh...
```

JWT 的 Payload 可以被客户端解析。

例如：

```json
{
  "sub": "123",
  "role": "admin"
}
```

但是：

**客户端能够解析，并不意味着客户端可以修改。**

服务器真正信任的是：

> **JWT 的数字签名（Signature）。**

而不是客户端解析出的 Payload。

---

# 按用途（Purpose）

## 1. Access Token

Access Token 用于访问受保护资源。

例如：

```http
Authorization: Bearer <Access Token>
```

特点：

* 生命周期较短
* 使用频率高
* 每次访问 API 都会使用

---

## 2. Refresh Token

Refresh Token 不用于访问业务资源。

它只用于：

> **获取新的 Access Token。**

通常：

```text
Access Token

↓

过期

↓

Refresh Token

↓

新的 Access Token
```

Refresh Token 通常具有更长的有效期。

---

# Access Token 与 Refresh Token

| Token 类型      | 有效期        | 用途                | 推荐存储位置                  |
| ------------- | ---------- | ----------------- | ----------------------- |
| Access Token  | 短（几分钟~几小时） | 访问 API            | 内存（Memory）              |
| Refresh Token | 长（几天~几个月）  | 获取新的 Access Token | HttpOnly Cookie 或其他安全存储 |

> 一般不推荐将 Access Token 保存在 `localStorage`，因为容易受到 XSS 攻击。

---

# Token 格式与用途的组合

由于：

* JWT / Opaque 是**格式**
* Access / Refresh 是**用途**

因此可以自由组合。

例如：

| Access Token | Refresh Token | 是否常见      |
| ------------ | ------------- | --------- |
| JWT          | Opaque        | ⭐⭐⭐⭐⭐ 最常见 |
| JWT          | JWT           | ⭐⭐⭐⭐      |
| Opaque       | Opaque        | ⭐⭐⭐       |
| Opaque       | JWT           | ⭐         |

其中：

**JWT Access Token + Opaque Refresh Token** 是目前最常见的方案。

原因：

* Access Token 使用 JWT，可以避免频繁查询数据库，提高性能。
* Refresh Token 使用 Opaque Token，方便立即失效、撤销以及控制登录状态。

---

# 总结

Bearer Authentication 是 HTTP 定义的一种认证方案（Authentication Scheme）。

HTTP 只规定：

```http
Authorization: Bearer <token>
```

至于 `<token>` 的内容是什么，HTTP 并不关心。

Token 可以按两个不同维度分类：

**按格式（Format）：**

* JWT
* Opaque Token

**按用途（Purpose）：**

* Access Token
* Refresh Token

因此：

* Access Token 可以是 JWT，也可以是 Opaque Token。
* Refresh Token 也可以是 JWT，也可以是 Opaque Token。

需要牢记：

> **Bearer 是 Authentication Scheme。**

> **JWT 是 Token Format。**

> **Access Token / Refresh Token 是 Token 的用途（Purpose）。**

它们属于不同层次的概念，不应混为一谈。
