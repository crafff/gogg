# JSON Web Token (JWT)

外部资源：

* https://jwt.io/introduction

---

# 什么是 JWT

JWT（JSON Web Token）是一种开放标准（RFC 7519），用于在各方之间**以可验证、防篡改的方式传递声明（Claims）**。

JWT 通常用于：

* 身份认证（Authentication）
* 授权（Authorization）
* 信息交换（Information Exchange）

需要注意的是：

> **JWT 默认并不加密（Encryption）。**

任何人都可以读取 JWT 中的 Header 和 Payload。

JWT 的安全性来自于**数字签名（Digital Signature）**，它能够保证：

* 数据没有被篡改（Integrity）
* JWT 确实由拥有签名密钥的一方签发（Authenticity）

JWT **默认不能保证数据的保密性（Confidentiality）**。

因此，不应该把密码、银行卡号等敏感数据直接放入 JWT Payload。

---

# JWT 的结构

JWT 由三部分组成：

* Header（头部）
* Payload（载荷）
* Signature（签名）

三部分之间使用 `.` 分隔。

例如：

```text
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9
.
eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ
.
SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
```

其中：

* Header 和 Payload 都是 JSON 对象，再经过 Base64Url 编码。
* Signature 是根据 Header 和 Payload 计算得到的数字签名。

---

## Header

Header 是一个 JSON 对象。

例如：

```json
{
  "alg": "HS256",
  "typ": "JWT"
}
```

其中：

* `alg`：签名算法（例如 HS256、RS256、ES256）
* `typ`：Token 类型，通常为 `"JWT"`

---

## Payload

Payload 包含一组 **Claims（声明）**。

Claim 本质上就是一组 **Key-Value**，用于描述 JWT 所代表的主体以及其他附加信息。

例如：

```json
{
  "sub": "1234567890",
  "name": "John Doe",
  "role": "admin"
}
```

这里：

* `"sub": "1234567890"` 是一个 Claim。
* `"name": "John Doe"` 也是一个 Claim。
* `"role": "admin"` 也是一个 Claim。

Claims 分为三类：

### 1. Registered Claims（注册声明）

Registered Claims 是 RFC 7519 已经定义好的标准字段。

例如：

* `iss`
* `sub`
* `aud`
* `exp`
* `iat`
* `nbf`
* `jti`

JWT 为了保持紧凑（Compact），这些字段名通常只有三个字符。

---

### 2. Public Claims（公共声明）

Public Claims 可以由开发者自定义。

为了避免不同系统之间发生名称冲突，应当：

* 使用 IANA 注册的 Claim 名称；或者
* 使用 URI 命名空间。

例如：

```json
{
  "https://example.com/role": "admin"
}
```

---

### 3. Private Claims（私有声明）

Private Claims 是通信双方自己约定的字段。

例如：

```json
{
  "role": "admin",
  "department": "R&D"
}
```

JWT 标准不会规定这些字段的含义，只要通信双方能够理解即可。

---

## Signature

Signature 用于保证 JWT 没有被篡改。

对于 HS256，其生成方式为：

```text
HMACSHA256(
    base64UrlEncode(header) + "." +
    base64UrlEncode(payload),
    secret
)
```

其中：

* Header
* Payload
* Secret

共同计算出 Signature。这里的 Secret 是签名密钥，只有签发 JWT 的一方和验证 JWT 的一方知道。如果是SH256算法，签名密钥是对称的；如果是RS256或ES256算法，签名密钥是非对称的 （public private key pair, 签名使用private key, 验证使用public key）。

需要注意：

**Signature 不是加密（Encryption）。**

它是一种**数字签名（Digital Signature）**。

JWT 的安全性正是建立在 Signature 的基础上。

---

# 常见 Registered Claims

---

## iss（Issuer）

表示 JWT 的签发者（Issuer）。

通常是签发 JWT 的认证服务器（Authentication Server）或者其他签发主体。

例如：

```json
{
  "iss": "https://auth.example.com"
}
```

---

## sub（Subject）

表示 JWT 所代表的主体（Subject）。

通常是用户的唯一 ID，也可以是服务、设备等其他实体。

例如：

```json
{
  "sub": "1234567890"
}
```

这里通常表示：

> 当前 JWT 代表用户 `1234567890`。

一般建议使用不会变化的唯一标识，而不是用户名。

---

## aud（Audience）

表示 JWT 的目标接收方（Audience）。

例如：

```json
{
  "aud": "https://api.example.com"
}
```

表示：

> 这张 JWT 是签发给 `https://api.example.com` 使用的。

如果其他 API 收到该 JWT，应当检查 `aud` 是否匹配，不匹配则拒绝访问。

---

## exp（Expiration Time）

表示 JWT 的过期时间。

它采用 **Unix 时间戳（NumericDate）**，单位为**秒**。

例如：

```json
{
  "exp": 1516239022
}
```

服务器应拒绝已经过期的 JWT。

---

# JWT 的使用流程

1. 用户登录（或使用 Refresh Token）。
2. 签发 JWT 的服务器（Issuer）生成 JWT，并返回给客户端。
3. 客户端在后续请求中，将 JWT 放入 HTTP Header。

例如：

```http
Authorization: Bearer <JWT>
```

4. 服务器验证 JWT。
5. 验证通过后，允许访问受保护资源。

---

# JWT 的验证流程

服务器收到 JWT 后，通常会执行以下步骤：

1. **解析 JWT**

   获取 Header、Payload 和 Signature。

2. **检查 Header**

   读取 Header 中的算法（`alg`）等信息。

3. **验证 Signature**

   根据 Header 中指定的算法验证签名。

   * HS256：使用同一个 Secret 验证。
   * RS256 / ES256：使用对应的 Public Key 验证。

   如果 Signature 不正确，则 JWT 无效。

4. **验证 Registered Claims**

   常见包括：

   * `iss`
   * `aud`
   * `exp`
   * `nbf`

5. **读取 Payload**

   当所有验证都通过后，服务器才会信任其中的 Claims。

---

# JWT 能保证什么？

JWT 能保证：

* ✅ Integrity（完整性）
* ✅ Authenticity（真实性）

JWT 默认不能保证：

* ❌ Confidentiality（保密性）

任何人都可以 Base64Url 解码 Header 和 Payload。

因此：

**不要把敏感数据直接存放在 JWT Payload 中。**

---

# 总结

JWT 由三部分组成：

```text
JWT
│
├── Header
│      alg
│      typ
│
├── Payload
│      Claims
│
└── Signature
```

其中：

* Header 描述 Token 类型和签名算法。
* Payload 包含 Claims（声明）。
* Signature 保证 JWT 没有被篡改，并证明它确实由拥有签名密钥的一方签发。

JWT 的安全性来自**数字签名（Digital Signature）**，而不是 Base64Url 编码。

JWT 默认能够保证：

* ✓ 完整性（Integrity）
* ✓ 来源真实性（Authenticity）

但默认不能保证：

* ✗ 保密性（Confidentiality）
