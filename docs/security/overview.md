• 这个 gogg 项目里已经用到的安全验证/安全防护知识点主要有这些：

  认证与会话

  1. OAuth2 第三方登录
      - 支持 Google、Discord Provider 抽象。
      - 通过授权码 code 换取用户信息。
      - 相关文件：
          - apps/api/internal/auth/provider/google.go
          - apps/api/internal/auth/provider/discord.go

  2. OAuth state 防 CSRF
      - /oauth/start/{provider} 生成 32 字节随机 state。
      - 存入 HttpOnly cookie。
      - /oauth/callback/{provider} 校验 query 中的 state 必须和 cookie 一致。
      - 校验后立即清除 state cookie，防止回放。
      - 相关文件：apps/api/internal/transport/rest/auth/auth.go

  3. JWT Access Token
      - 使用 HS256 签名。
      - 包含 sub、iss、iat、nbf、exp、jti。
      - 校验签名、issuer、有效期、算法。
      - 明确限制只接受 HS256，防止算法混淆。
      - 相关文件：apps/api/internal/auth/jwt.go

  4. JWT 密钥强度校验
      - jwt_secret 长度必须至少 32 字节。
      - access / refresh TTL 必须大于 0。
      - refresh TTL 必须长于 access TTL。
      - 相关文件：apps/api/internal/auth/jwt.go

  5. Bearer Token 解析
      - 只接受 Authorization: Bearer <token>。
      - scheme 大小写不敏感。
      - 空 token、非 Bearer token 会被拒绝解析。
      - 相关文件：apps/api/internal/transport/middleware/auth.go

  6. 短期 Access Token + 长期 Refresh Token 模型
      - Access token 默认 15 分钟。
      - Refresh token 默认 30 天。
      - Access token 放在响应体，由前端用 Authorization 携带。
      - Refresh token 放在 cookie。
      - 相关文件：apps/api/internal/auth/jwt.go

  7. Opaque Refresh Token
      - Refresh token 不是 JWT，而是 crypto/rand 生成的 256-bit 随机字符串。
      - 使用 Base64 URL safe 编码。
      - 相关文件：apps/api/internal/auth/refresh.go

  8. Refresh Token 哈希入库
      - 数据库只保存 sha256(refresh_token)。
      - 明文 refresh token 只存在于 cookie / 请求中。
      - 数据库泄露时，攻击者不能直接拿 token 使用。
      - 相关文件：
          - apps/api/internal/auth/jwt.go
          - packages/sqlc/migrations/013_users.up.sql

  9. Refresh Token 轮换
      - /auth/refresh 使用旧 refresh token 后立即 revoke。
      - 然后签发新的 access token 和新的 refresh token。
      - 旧 refresh token 一次性使用。
      - 相关文件：apps/api/internal/service/user/service.go

  10. Refresh Token 过期与撤销校验
      - 校验 revoked_at。
      - 校验 expires_at。
      - 找不到、过期、已撤销都返回无效。
      - 相关文件：apps/api/internal/service/user/service.go

  11. Logout 撤销刷新令牌
      - /auth/logout revoke 当前 refresh token。
      - 清除 cookie。
      - 幂等处理，无 cookie 也返回成功。
      - 相关文件：apps/api/internal/transport/rest/auth/auth.go

  12. HttpOnly Cookie
      - Refresh token cookie 设置 HttpOnly，降低 XSS 直接读取 token 的风险。
      - 相关文件：apps/api/internal/transport/rest/auth/auth.go

  13. SameSite Cookie
      - OAuth state cookie 和 refresh cookie 都使用 SameSite=Lax。
      - 用于降低跨站请求风险。
      - 相关文件：apps/api/internal/transport/rest/auth/auth.go

  14. Secure Cookie 配置
      - cookie_secure 可配置。
      - 本地 HTTP 可关闭，生产 HTTPS 应开启。
      - 相关文件：
          - apps/api/internal/config/config.go
          - apps/api/internal/transport/rest/auth/auth.go

  15. Cookie Domain 最小化
      - 默认空 domain，即 host-only cookie。
      - 避免 cookie 泄露给不必要的子域。
      - 相关文件：apps/api/internal/transport/rest/auth/auth.go

  授权与身份绑定

  16. 用户 ID 使用 UUID
      - 用户主键为 UUID。
      - JWT sub 必须能解析为 UUID。
      - 相关文件：
          - apps/api/internal/auth/jwt.go
          - packages/sqlc/migrations/013_users.up.sql

  17. OAuth 身份唯一绑定
      - (provider, provider_user_id) 唯一。
      - 一个外部账号只能绑定到一个 gogg 用户。
      - 相关文件：packages/sqlc/migrations/013_users.up.sql

  18. 未配置 OAuth Provider 不注册
      - client_id、client_secret、redirect_url 不完整时跳过。
      - 请求未知 provider 返回 404。
      - 相关文件：apps/api/cmd/api/main.go

  输入校验与参数约束

  19. REST 查询参数默认值
      - queueId、minGames、limit、positionThreshold 等都有默认值。
      - 相关文件：apps/api/internal/transport/rest/v1/rankings.go

  20. REST 查询参数范围夹紧
      - limit 限制在 [-1, 500]。
      - minGames 限制在 [1, 20000]。
      - queueId 限制在 [0, 9999]。
      - positionThreshold 限制在 [0, 100]。
      - 相关文件：apps/api/internal/transport/rest/v1/rankings.go

  21. 字符串规范化
      - region 转大写。
      - position 转大写。
      - tier 转小写。
      - version trim 空白。
      - 相关文件：
          - apps/api/internal/transport/rest/v1/rankings.go
          - apps/api/internal/transport/graphql/resolver/rankings.resolvers.go

  22. GraphQL 类型约束
      - GraphQL schema 对字段类型做约束，例如 Int、Float、TierGroup enum。
      - 相关文件：apps/api/internal/transport/graphql/schema/rankings.graphql

  23. GraphQL 输入参数范围夹紧
      - GraphQL filter 和 REST 一样做 clamp。
      - 相关文件：apps/api/internal/transport/graphql/resolver/rankings.resolvers.go

  24. 配置启动校验
      - 校验端口范围。
      - 校验超时时间必须大于 0。
      - 校验数据库 DSN 非空。
      - 校验连接池数量。
      - 校验日志 level / format 枚举值。
      - 相关文件：apps/api/internal/config/config.go

  跨域与浏览器安全

  25. CORS 白名单
      - 只允许配置中的 exact origin。
      - 不支持 * 通配。
      - 未匹配 origin 不回显 Access-Control-Allow-Origin。
      - 相关文件：apps/api/internal/transport/middleware/cors.go

  26. CORS Vary Header
      - 设置 Vary: Origin，避免缓存污染不同 Origin 的响应。
      - 相关文件：apps/api/internal/transport/middleware/cors.go

  27. CORS 方法和头限制
      - 只允许 GET, POST, OPTIONS。
      - 只允许 Content-Type, Authorization, X-Request-Id。
      - 相关文件：apps/api/internal/transport/middleware/cors.go

  28. 前端安全响应头
      - X-Content-Type-Options: nosniff
      - X-Frame-Options: DENY
      - Referrer-Policy: strict-origin-when-cross-origin
      - 相关文件：deploy/docker/nginx.conf

  错误处理与信息泄露控制

  29. panic 恢复
      - middleware 捕获 panic。
      - 服务端记录 stack。
      - 客户端只返回通用 internal server error。
      - 相关文件：apps/api/internal/transport/middleware/recover.go

  30. GraphQL 错误脱敏
      - 内部错误不直接返回 err.Error()。
      - 默认返回 internal server error。
      - 只有 domainerr.Error 这种显式标记安全的错误才暴露给客户端。
      - 相关文件：
          - apps/api/internal/transport/graphql/server.go
          - apps/api/internal/transport/graphql/domainerr/domainerr.go

  31. REST 错误响应脱敏
      - 认证失败、OAuth 失败、排名查询失败返回固定安全消息。
      - 真实错误进入日志。
      - 相关文件：
          - apps/api/internal/transport/rest/auth/auth.go
          - apps/api/internal/transport/rest/v1/rankings.go

  32. 请求 ID
      - 支持 X-Request-Id。
      - 没有就用 crypto/rand 生成。
      - 响应中回传，方便审计和排查。
      - 相关文件：apps/api/internal/transport/middleware/request_id.go

  33. 结构化请求日志
      - 记录 request id、method、path、status、bytes、duration。
      - 相关文件：apps/api/internal/transport/middleware/logger.go

  34. Prometheus 指标防高基数攻击
      - metrics 使用 chi route pattern。
      - 未匹配路由统一归为 unmatched。
      - 避免攻击者用随机 URL 撑爆指标标签。
      - 相关文件：apps/api/internal/transport/middleware/metrics.go

  SQL 与数据层安全

  35. sqlc 参数化查询
      - SQL 使用 $1、$2 参数。
      - Go 代码调用 sqlc 生成方法，不拼接 SQL 字符串。
      - 降低 SQL 注入风险。
      - 相关文件：packages/sqlc/queries/users.sql

  36. 数据库唯一约束
      - refresh token hash 唯一。
      - OAuth identity 唯一。
      - 相关文件：packages/sqlc/migrations/013_users.up.sql

  37. 外键级联删除
      - OAuth identities 和 refresh tokens 通过 user_id 关联用户。
      - 用户删除后相关凭证自动删除。
      - 相关文件：packages/sqlc/migrations/013_users.up.sql

  服务端运行安全

  38. HTTP 超时
      - ReadTimeout
      - ReadHeaderTimeout
      - WriteTimeout
      - IdleTimeout
      - 可缓解慢请求/连接耗尽类问题。
      - 相关文件：apps/api/cmd/api/main.go

  39. 优雅关闭
      - 捕获 SIGINT / SIGTERM。
      - 使用 ShutdownGrace 控制关闭窗口。
      - 相关文件：apps/api/cmd/api/main.go

  40. 健康检查与就绪检查
      - /healthz
      - /readyz
      - 数据库、Redis ping。
      - 初始化失败时返回 503，避免错误实例接流量。
      - 相关文件：apps/api/cmd/api/main.go

  41. GraphQL 查询复杂度限制
      - FixedComplexityLimit(300)。
      - 防止复杂查询拖垮服务。
      - 相关文件：apps/api/internal/transport/graphql/server.go

  42. GraphQL Query Cache / Persisted Query
      - 使用 LRU cache。
      - Automatic Persisted Query cache 限制为 100。
      - 相关文件：apps/api/internal/transport/graphql/server.go

  43. GraphQL Playground 可配置
      - 只有配置开启时挂载 /graphql/playground。
      - 生产应关闭。
      - 相关文件：
          - apps/api/cmd/api/main.go
          - apps/api/internal/transport/graphql/server.go

  外部 API 与限流

  44. Riot API 客户端限流
      - 同时限制每秒请求数和每 2 分钟请求数。
      - 使用 golang.org/x/time/rate。
      - 相关文件：packages/riotapi/limiter.go

  45. 外部 API HTTPS
      - Riot、Google、Discord endpoint 都使用 HTTPS。
      - 相关文件：
          - apps/api/internal/auth/provider/google.go
          - apps/api/internal/auth/provider/discord.go
          - config/worker.example.yaml

  密钥与配置安全

  46. SOPS + age 管理密钥
      - deploy/secrets/*.enc.yaml 存放加密后的环境密钥。
      - 不把明文密钥提交进 git。
      - 相关文件：deploy/secrets/README.md

  47. 环境变量覆盖配置
      - 支持 GOGG_* 环境变量覆盖配置。
      - 适合部署环境注入密钥。
      - 相关文件：apps/api/internal/config/config.go

  48. 不注册空 OAuth 凭证
      - 避免半配置状态暴露错误 provider。
      - 相关文件：apps/api/cmd/api/main.go

  49. 密钥扫描意识
      - lefthook.yml 中有避免 secrets 进入 git 的检查说明。
      - 项目文档也强调 secrets 不落 git。
      - 相关文件：
          - lefthook.yml
          - CLAUDE.md

  当前未完全落地但代码/注释中提到的安全规划

  50. RS256 + Key Rotation
      - 当前是 HS256。
      - 注释中提到后续升级到 RS256 和密钥轮换。
      - 相关文件：apps/api/internal/auth/jwt.go

  51. PKCE
      - 当前 OAuth flow 用 state 防 CSRF。
      - 注释提到后续增加 PKCE。
      - 相关文件：apps/api/internal/transport/rest/auth/auth.go

  52. Access Token denylist
      - JWT 中已有 jti。
      - 注释提到未来可用于单个 access token 撤销。
      - 当前尚未实现 denylist。
      - 相关文件：apps/api/internal/auth/jwt.go

  53. 可信代理 hop 配置
      - 当前 clientIP 直接信任 X-Forwarded-For 第一项。
      - 注释提到后续增加“信任多少层代理”的配置。
      - 相关文件：apps/api/internal/transport/rest/auth/auth.go

  54. 更严格 CSP
      - 当前 nginx 有基础安全头。
      - 注释提到 CSP 后续加强。
      - 相关文件：deploy/docker/nginx.conf

  总结：这个项目覆盖了认证、OAuth 防 CSRF、JWT 校验、刷新令牌轮换、Cookie 安全、CORS 白名单、输入范围校验、GraphQL
  复杂度限制、SQL 参数化、错误脱敏、密钥管理、服务端超时、限流和审计日志等安全验证知识点。当前比较核心的是 OAuth +
  JWT + opaque refresh token rotation + HttpOnly cookie + CORS allowlist + sqlc 参数化查询 + GraphQL/REST 错误脱敏