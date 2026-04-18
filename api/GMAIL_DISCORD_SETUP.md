# Gmail / Google / Discord 配置指引

这份文档只说明当前 `api` 项目里，Gmail、Google OAuth、Discord OAuth 应该在哪处理，以及上线前必须检查的点。

## 先说结论

- 如果你说的 `gmail` 是“发注册验证邮件”:
  处理位置在 `api/internal/handlers/auth.go` 的 `sendVerificationEmail()`
- 如果你说的 `gmail` 是“用 Google 账号登录”:
  处理位置也在 `api/internal/handlers/auth.go`，走 `google` OAuth 分支
- 如果你说的 `dc` 是 Discord:
  当前项目里对应的是 Discord OAuth，处理位置同样在 `api/internal/handlers/auth.go`
- 所有配置值都从环境变量进入，统一读取入口在 `api/internal/core/config/config.go`
- Docker 启动时，`api` 服务会读取 `api/.env`，见项目根目录 `docker-compose.yml`
- API 域名现在也已支持变量配置，使用 `API_BASE_URL`

## 当前代码里的关键位置

### 1. 环境变量入口

文件:

- `api/internal/core/config/config.go`

这里已经支持下面这些变量:

- `API_BASE_URL`
- `FRONTEND_URL`
- `GOOGLE_CLIENT_ID`
- `GOOGLE_CLIENT_SECRET`
- `DISCORD_CLIENT_ID`
- `DISCORD_CLIENT_SECRET`
- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_USER`
- `SMTP_PASS`
- `SMTP_FROM`

### 2. 路由入口

文件:

- `api/cmd/server/main.go`

已经注册的认证相关路由:

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/auth/verify-email`
- `GET /api/v1/auth/oauth/:provider`
- `GET /api/v1/auth/oauth/:provider/callback`

其中 `:provider` 目前支持:

- `google`
- `discord`

### 3. 邮件发送逻辑

文件:

- `api/internal/handlers/auth.go`

关键函数:

- `sendVerificationEmail(email, token string)`

当前行为:

- `SMTP_HOST` 为空时，不发邮件，只把验证链接打印到日志
- `SMTP_HOST` 有值时，使用 `net/smtp` 发送验证邮件

### 4. Google / Discord OAuth 逻辑

文件:

- `api/internal/handlers/auth.go`

关键函数:

- `OAuthRedirect`
- `OAuthCallback`
- `exchangeGoogleCode`
- `fetchGoogleUserInfo`
- `exchangeDiscordCode`
- `fetchDiscordUserInfo`

### 5. 前端登录回跳页

文件:

- `frontend/pages/auth/callback.vue`

说明:

- 邮箱验证成功后，后端会跳到前端 `/auth/callback?token=...`
- Google / Discord OAuth 成功后，也会跳到这个页面

## 你应该改哪里

### A. 配 Gmail 发信

如果目标是“注册后发验证邮件”，你主要处理:

- `api/.env`
- `api/internal/core/config/config.go`
- `api/internal/handlers/auth.go`

实际配置写在 `api/.env`，示例:

```env
API_BASE_URL=http://localhost:8080
FRONTEND_URL=http://localhost:3000

SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your_account@gmail.com
SMTP_PASS=your_gmail_app_password
SMTP_FROM=your_account@gmail.com
```

注意:

- Gmail 通常不能直接用邮箱登录密码，应该使用 App Password
- 当前代码使用 `smtp.PlainAuth`，因此推荐走 `587` + TLS/STARTTLS 场景
- 这部分现在只是“SMTP 发信”，不是 Google OAuth 登录

## B. 配 Google 登录

如果你的“gmail”实际是“Google 账号登录”，你主要处理:

- `api/.env`
- `api/internal/core/config/config.go`
- `api/internal/handlers/auth.go`

环境变量:

```env
API_BASE_URL=http://localhost:8080
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret
FRONTEND_URL=http://localhost:3000
```

当前后端登录入口:

```text
GET /api/v1/auth/oauth/google
```

Google OAuth 回调地址要和代码保持一致:

- 本地开发: `http://localhost:8080/api/v1/auth/oauth/google/callback`
- 生产环境: `<API_BASE_URL>/api/v1/auth/oauth/google/callback`

## C. 配 Discord 登录

如果你的 `dc` 指 Discord，你主要处理:

- `api/.env`
- `api/internal/core/config/config.go`
- `api/internal/handlers/auth.go`

环境变量:

```env
API_BASE_URL=http://localhost:8080
DISCORD_CLIENT_ID=your_discord_client_id
DISCORD_CLIENT_SECRET=your_discord_client_secret
FRONTEND_URL=http://localhost:3000
```

当前后端登录入口:

```text
GET /api/v1/auth/oauth/discord
```

Discord OAuth 回调地址要和代码保持一致:

- 本地开发: `http://localhost:8080/api/v1/auth/oauth/discord/callback`
- 生产环境: `<API_BASE_URL>/api/v1/auth/oauth/discord/callback`

## `api/.env` 建议补齐的配置块

当前仓库的 `api/.env.example` 里还没有把这些变量列全，实际使用时建议在 `api/.env` 至少补上:

```env
# API
API_BASE_URL=http://localhost:8080

# Frontend
FRONTEND_URL=http://localhost:3000

# Google OAuth
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=

# Discord OAuth
DISCORD_CLIENT_ID=
DISCORD_CLIENT_SECRET=

# SMTP / Gmail
SMTP_HOST=
SMTP_PORT=587
SMTP_USER=
SMTP_PASS=
SMTP_FROM=noreply@clawlink.app
```

## 现在的地址配置方式

邮件验证链接基础地址和 OAuth callback 基础地址现在都走:

```env
API_BASE_URL=https://your-api-domain.com
```

对应代码:

- `oauthCallbackURL()`
- `sendVerificationEmail()`

如果你的线上 API 域名不是默认值，只需要改 `api/.env` 里的 `API_BASE_URL`。

## 本地开发怎么判断是否生效

### 验证 Gmail SMTP

- 调用注册接口 `POST /api/v1/auth/register`
- 如果 `SMTP_HOST` 为空，终端日志会输出验证链接
- 如果 `SMTP_HOST` 已配置，应该实际收到验证邮件

### 验证 Google OAuth

- 打开 `GET /api/v1/auth/oauth/google`
- 应该跳转到 Google 授权页
- 完成后会回到前端 `/auth/callback`

### 验证 Discord OAuth

- 打开 `GET /api/v1/auth/oauth/discord`
- 应该跳转到 Discord 授权页
- 完成后会回到前端 `/auth/callback`

## 推荐的处理顺序

1. 先在 `api/.env` 补齐 `FRONTEND_URL`、SMTP、Google、Discord 变量
2. 确认你的真实前后端域名
3. 配好 `API_BASE_URL`
4. 再去 Google Console 和 Discord Developer Portal 填对应回调地址
5. 最后分别测试邮件验证、Google 登录、Discord 登录

## 一句话判断

- 发 Gmail 邮件: 重点看 `sendVerificationEmail()`
- Google 登录: 重点看 `OAuthRedirect/OAuthCallback` 的 `google` 分支
- Discord 登录: 重点看 `OAuthRedirect/OAuthCallback` 的 `discord` 分支
- 统一配置入口: `config.go`
- 统一实际填写位置: `api/.env`
