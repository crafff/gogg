• 这个 gogg 项目里已经用到的安全验证/安全防护知识点主要有这些：

  认证与会话

  1. Google OAuth2 + PKCE
      - 浏览器登录只启用 Google；Discord Provider 保留但不注册，Riot RSO 等待审批。
      - 授权请求使用随机 state 和 S256 PKCE，换码在服务端完成。
      - 相关文件：
          - apps/api/internal/auth/provider/google.go
          - apps/api/internal/service/user/service.go

  2. 一次性 OAuth 登录尝试
      - state 和浏览器绑定值只以 SHA-256 哈希入库。
      - `UPDATE ... RETURNING` 原子消费未过期 attempt，回调无法重放。
      - state、provider、浏览器绑定和 PKCE verifier 一起约束同一次登录。
      - 相关文件：
          - packages/sqlc/migrations/024_browser_sessions.up.sql
          - packages/sqlc/queries/users.sql

  3. 服务端 opaque browser session
      - SPA 不接收 Google token、GOGG JWT 或 refresh token。
      - cookie 中是 crypto/rand 生成的 256-bit 随机 secret，数据库只保存 SHA-256。
      - 查询同时校验 `revoked_at IS NULL` 和 `expires_at > now()`。
      - 相关文件：
          - apps/api/internal/auth/jwt.go
          - apps/api/internal/service/user/service.go
          - packages/sqlc/migrations/024_browser_sessions.up.sql

  4. HttpOnly / Secure / SameSite Cookie
      - 会话 cookie 为 host-only、`Path=/`、HttpOnly、SameSite=Strict。
      - HTTPS 环境使用 Secure 和 `__Host-gogg_session`；本地 HTTP 使用普通名称。
      - OAuth 浏览器绑定 cookie 为 HttpOnly、SameSite=Lax、`Path=/oauth`，仅保留 10 分钟。
      - 相关文件：apps/api/internal/transport/rest/auth/auth.go

  5. Cookie 请求 CSRF 防护
      - 携带会话 cookie 的非安全方法必须带 `X-GOGG-CSRF: 1`。
      - 跨站表单无法设置该自定义头，CORS 又只允许精确 origin。
      - 相关文件：
          - apps/api/internal/transport/middleware/session.go
          - apps/api/internal/transport/middleware/cors.go

  6. 安全退出
      - `/auth/logout` 先撤销服务端 session，再清除浏览器 cookie。
      - 数据库失败时保留 cookie 并返回 503，避免把未撤销误报为成功。
      - 无 cookie 的退出保持幂等。

  7. 开放重定向与输入边界
      - `returnTo` 只接受长度受限的站内绝对路径，拒绝 scheme、host、双斜杠和反斜杠。
      - callback 的 code、state、浏览器绑定、User-Agent 和 provider profile 都有大小上限。
      - OAuth/登录失败只返回固定错误码，不回显 provider 或数据库错误。

  8. 可选 Bearer JWT 兼容
      - 非浏览器客户端仍可使用 HS256 Bearer 验证；浏览器 Google 登录不签发 JWT。
      - 只接受 Bearer scheme 和固定算法，并校验 issuer、有效期和 UUID subject。
      - 相关文件：
          - apps/api/internal/auth/jwt.go
          - apps/api/internal/transport/middleware/auth.go

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
      - 只允许 Content-Type、Authorization、X-GOGG-CSRF、X-Request-Id。
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
      - browser session token hash 唯一。
      - OAuth identity 唯一。
      - 相关文件：packages/sqlc/migrations/013_users.up.sql、024_browser_sessions.up.sql

  37. 外键级联删除
      - OAuth identities 和 browser sessions 通过 user_id 关联用户。
      - 用户删除后相关凭证自动删除。
      - 相关文件：packages/sqlc/migrations/013_users.up.sql、024_browser_sessions.up.sql

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
  Google OAuth PKCE + opaque server session + HttpOnly cookie + CSRF header + CORS allowlist + sqlc 参数化查询 + GraphQL/REST 错误脱敏
