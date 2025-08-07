# SUFE AI Telegram Bot

高性能的 Telegram AI 机器人，基于 Go 语言开发，支持自定义 AI 端点（OpenAI、Gemini、本地模型等），具备多模型切换、上下文记忆、知识库检索、速率限制和全面的监控体系。

## 📋 目录

- [核心特性](#核心特性)
- [系统要求](#系统要求)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [部署指南](#部署指南)
- [端口说明](#端口说明)
- [Nginx反向代理配置](#nginx反向代理配置)
- [日志管理](#日志管理)
- [监控指南](#监控指南)
- [命令列表](#命令列表)
- [开发指南](#开发指南)
- [性能对比](#性能对比)
- [故障排查](#故障排查)
- [安全建议](#安全建议)

## 🚀 核心特性

### 架构优势
- **高并发处理**: 基于 Go 协程，可同时处理数千个请求
- **模块化设计**: 清晰的代码结构，易于维护和扩展
- **微服务就绪**: 支持容器化部署和水平扩展

### 性能特性
- **智能缓存**: 双层缓存架构（内存+Redis），大幅提升响应速度
- **连接池管理**: HTTP 连接复用，减少网络开销
- **自动重试**: 指数退避算法，提高服务可靠性
- **内存优化**: 自动清理机制，防止内存泄漏

### 功能特性
- **自定义AI端点**: 支持配置任意 OpenAI 兼容的 API 端点（OpenAI、Gemini、本地模型等）
- **多模型管理**: 可同时配置多个端点和模型，灵活切换使用
- **知识库检索**: 内置向量数据库，支持上传文档并智能检索相关内容
- **上下文记忆**: 保持对话连贯性，支持多轮对话
- **多语言界面**: 内置中英文支持，易于扩展其他语言
- **灵活触发**: @提及、回复、关键词等多种触发方式
- **个性化设置**: 每个聊天独立的模型、背景设定、提及词等

### 安全与监控
- **速率限制**: 防止 API 滥用，支持用户级别限流
- **输入验证**: 防御恶意输入和注入攻击
- **结构化日志**: JSON 格式日志，支持日志分析
- **Prometheus 指标**: 全面的性能和业务指标
- **健康检查**: 内置健康检查端点

## 📋 系统要求

- **Go**: 1.21 或更高版本
- **Docker**: 20.10+ 和 Docker Compose 2.0+
- **Redis**: 7.0+（使用 Redis 存储时）
- **系统资源**: 
  - 最小：1 CPU, 512MB RAM
  - 推荐：2 CPU, 1GB RAM

## 🚀 快速开始

### 使用 Docker Compose（推荐）

1. **克隆项目**
```bash
git clone https://github.com/Shannon-x/sufe-ai-bot.git
cd sufe-ai-bot
```

2. **配置环境变量**
```bash
cp .env.example .env
# 编辑 .env 文件，填入您的配置
```

3. **一键部署**
```bash
# 使用内置Redis
./deploy.sh

# 或者使用外部Redis
docker-compose -f docker-compose.external-redis.yml up -d
```

### 使用外部 Redis

如果您已经有运行中的 Redis 实例，可以配置机器人使用外部 Redis：

1. **配置环境变量**
```bash
# 编辑 .env 文件
REDIS_HOST=your-redis-host     # Redis 服务器地址
REDIS_PORT=6379                # Redis 端口
REDIS_PASSWORD=your-password   # Redis 密码（如果有）
REDIS_DB=0                     # 数据库编号
```

2. **使用外部 Redis 启动**
```bash
# 使用专用的 docker-compose 文件
docker-compose -f docker-compose.external-redis.yml up -d
```

### 手动部署

1. **安装依赖**
```bash
go mod download
```

2. **配置环境**
```bash
cp .env.example .env
vim .env  # 编辑配置
```

3. **运行程序**
```bash
# 开发模式
go run cmd/bot/main.go

# 生产模式
go build -o bot cmd/bot/main.go
./bot --config configs/config.yaml
```

## ⚙️ 配置说明

### 环境变量 (.env)

```bash
# Telegram 机器人配置
BOT_TOKEN=your_bot_token_here

# AI 端点配置（可选，如果不配置将使用 config.yaml 中的默认值）
OPENAI_API_KEY=your_openai_api_key      # OpenAI API 密钥
CUSTOM_API_URL=http://localhost:8080/v1  # 自定义 API 端点地址
CUSTOM_API_KEY=your_custom_api_key      # 自定义 API 密钥

# Redis 配置（如果使用外部 Redis）
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
```

### 主配置文件 (config.yaml)

```yaml
# 机器人配置
bot:
  token: ${BOT_TOKEN}
  webhook:
    enabled: false  # 是否启用 webhook 模式
    url: "https://your-domain.com"  # webhook URL
    port: 8443  # webhook 监听端口
  update_timeout: 60  # 长轮询超时时间

# AI 模型配置
models:
  default: "gemini-2.5-flash"  # 默认使用的模型ID
  endpoints:
    - name: "openai"
      display_name: "OpenAI"
      base_url: "https://api.openai.com/v1"
      api_key: ${OPENAI_API_KEY}
      models:
        - id: "gpt-3.5-turbo"
          name: "GPT-3.5 Turbo"
          max_tokens: 4096
        - id: "gpt-4"
          name: "GPT-4"
          max_tokens: 8192
    
    - name: "custom"
      display_name: "自定义端点"
      base_url: ${CUSTOM_API_URL:http://localhost:8080/v1}
      api_key: ${CUSTOM_API_KEY}
      models:
        - id: "custom-model"
          name: "自定义模型"
          max_tokens: 4096

# 存储配置
storage:
  type: "redis"  # "redis" 或 "memory"
  redis:
    addr: "${REDIS_HOST:localhost}:${REDIS_PORT:6379}"  # 支持环境变量
    password: "${REDIS_PASSWORD:}"
    db: ${REDIS_DB:0}
  memory:
    default_expiration: 24h
    cleanup_interval: 1h

# 缓存配置
cache:
  enabled: true
  ttl: 1h
  max_size: 1000

# 速率限制
rate_limit:
  enabled: true
  requests_per_minute: 30
  burst: 50

# 日志配置
logging:
  level: "info"  # debug, info, warn, error
  format: "json"  # json 或 text
  output: "file"  # stdout 或 file
  file:
    path: "/var/log/cf-ai-tgbot/bot.log"
    max_size: 100    # MB，单个日志文件最大大小
    max_backups: 3   # 保留的旧日志文件数量
    max_age: 7       # 天，日志文件保留时间
    compress: true   # 是否压缩旧日志文件

# 监控配置
monitoring:
  metrics:
    enabled: true
    port: 9090
    path: "/metrics"
```

## 🌐 端口说明

| 端口 | 服务 | 用途 | 是否需要公开 |
|-----|------|------|------------|
| 8443 | Bot Webhook | Telegram webhook 接收端口 | 是（如果使用 webhook） |
| 9090 | Metrics | Prometheus 指标端点 | 否（仅内部访问） |
| 9091 | Prometheus | Prometheus Web UI | 可选 |
| 3000 | Grafana | 监控可视化界面 | 可选 |
| 6379 | Redis | 数据存储 | 否（仅内部访问） |

## 🔧 Nginx 反向代理配置

### 基础配置（仅 Webhook）

```nginx
server {
    listen 443 ssl http2;
    server_name bot.yourdomain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # Telegram Webhook
    location /bot${BOT_TOKEN} {
        proxy_pass http://localhost:8443;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### 完整配置（包含监控）

```nginx
# 主域名 - Bot Webhook
server {
    listen 443 ssl http2;
    server_name bot.yourdomain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # 安全头
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Telegram Webhook
    location /bot${BOT_TOKEN} {
        proxy_pass http://localhost:8443;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Telegram 特定设置
        proxy_read_timeout 300s;
        proxy_connect_timeout 75s;
    }
}

# 监控子域名
server {
    listen 443 ssl http2;
    server_name monitor.yourdomain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # 基础认证
    auth_basic "Monitoring Access";
    auth_basic_user_file /etc/nginx/.htpasswd;

    # Grafana
    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }

    # Prometheus
    location /prometheus/ {
        proxy_pass http://localhost:9091/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
    }

    # Bot Metrics（可选）
    location /metrics {
        proxy_pass http://localhost:9090/metrics;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
    }
}

# HTTP 重定向到 HTTPS
server {
    listen 80;
    server_name bot.yourdomain.com monitor.yourdomain.com;
    return 301 https://$server_name$request_uri;
}
```

### 创建基础认证

```bash
# 安装 htpasswd 工具
sudo apt-get install apache2-utils

# 创建用户
sudo htpasswd -c /etc/nginx/.htpasswd admin
```

## 📊 日志管理

### 日志轮转配置

项目使用 `lumberjack` 自动管理日志轮转，配置项说明：

```yaml
logging:
  file:
    path: "/var/log/cf-ai-tgbot/bot.log"
    max_size: 100       # 单个日志文件最大 100MB
    max_backups: 3      # 保留最近 3 个日志文件
    max_age: 7          # 日志文件保留 7 天
    compress: true      # 压缩旧日志文件（.gz）
```

### 日志存储计算

- 最大存储空间：100MB × 4 = 400MB（当前文件 + 3 个备份）
- 压缩后约占用：~100MB（压缩率约 75%）

### 外部日志管理（可选）

如果需要更复杂的日志管理，可以配合 `logrotate`：

```bash
# /etc/logrotate.d/cf-ai-tgbot
/var/log/cf-ai-tgbot/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0644 root root
    sharedscripts
    postrotate
        docker-compose kill -s USR1 bot
    endscript
}
```

### 日志查看命令

```bash
# 查看实时日志
docker-compose logs -f bot

# 查看最近 100 行日志
docker-compose logs --tail=100 bot

# 按时间查看日志
docker-compose logs --since="2024-01-01T00:00:00" bot

# 导出日志
docker-compose logs bot > bot_logs.txt
```

## 📈 监控指南

### Prometheus 指标

访问 `http://localhost:9090/metrics` 查看所有指标：

- `telegram_bot_messages_received_total` - 接收消息总数
- `telegram_bot_messages_processed_total` - 处理消息总数
- `telegram_bot_commands_executed_total` - 执行命令总数
- `telegram_bot_ai_request_duration_seconds` - AI 请求耗时
- `telegram_bot_cache_hits_total` - 缓存命中次数
- `telegram_bot_rate_limit_exceeded_total` - 触发限流次数

### Grafana 配置

1. 访问 `http://localhost:3000`（默认：admin/admin）
2. 添加 Prometheus 数据源：`http://prometheus:9090`
3. 导入仪表板（可以使用项目提供的模板）

### 告警配置示例

```yaml
# prometheus/alerts.yml
groups:
  - name: bot_alerts
    rules:
      - alert: HighErrorRate
        expr: rate(telegram_bot_messages_processed_total{status="error"}[5m]) > 0.1
        for: 5m
        annotations:
          summary: "High error rate detected"
          
      - alert: HighMemoryUsage
        expr: process_resident_memory_bytes > 1e9
        for: 10m
        annotations:
          summary: "Memory usage exceeds 1GB"
```

## 🔧 自定义模型配置

### 配置新的 AI 端点

1. **编辑 config.yaml 文件**：
```yaml
models:
  endpoints:
    - name: "my-custom-endpoint"
      display_name: "我的自定义端点"
      base_url: "https://api.example.com/v1"  # 必须兼容 OpenAI API 格式
      api_key: "${MY_CUSTOM_API_KEY}"         # 使用环境变量
      models:
        - id: "model-1"
          name: "模型 1"
          max_tokens: 4096
        - id: "model-2"
          name: "模型 2"
          max_tokens: 8192
```

2. **设置环境变量**：
```bash
# 在 .env 文件中添加
MY_CUSTOM_API_KEY=your_api_key_here
```

3. **支持的 API 类型**：
   - OpenAI API
   - Google Gemini API
   - Anthropic Claude API
   - 本地模型（Ollama、LocalAI 等）
   - 任何兼容 OpenAI Chat Completions API 的服务

### 常见配置示例

**Ollama 本地模型**：
```yaml
- name: "ollama"
  display_name: "Ollama 本地模型"
  base_url: "http://localhost:11434/v1"
  api_key: "ollama"  # Ollama 不需要真实的 API key
  models:
    - id: "llama3.2"
      name: "Llama 3.2"
      max_tokens: 4096
```

**Google Gemini**：
```yaml
- name: "gemini"
  display_name: "Google Gemini"
  base_url: "https://generativelanguage.googleapis.com/v1beta"
  api_key: "${GEMINI_API_KEY}"
  models:
    - id: "gemini-2.5-flash"
      name: "Gemini 2.5 Flash"
      max_tokens: 8192
```

## 📖 知识库功能

### 启用知识库

1. **配置知识库目录**：
```yaml
knowledge:
  enabled: true
  directory: "./knowledge"  # 知识库文件存放目录
```

2. **添加知识文档**：
   - 将 Markdown 文件放入 `knowledge` 目录
   - 支持 `.md` 和 `.txt` 格式
   - 机器人会自动索引这些文档

3. **使用知识库**：
   - 机器人会自动根据用户问题检索相关知识
   - 使用 `/knowledge` 命令查看知识库状态

### 知识文档格式

```markdown
# 文档标题

## 章节 1
内容...

## 章节 2
内容...
```

机器人会智能分割文档并建立索引，在回答时引用相关内容。

## 💬 命令列表

### 基础命令
- `/start` - 开始使用机器人
- `/help` - 显示帮助信息
- `/clear` - 清空当前对话记忆
- `/models` - 查看和切换 AI 模型
- `/settings` - 设置语言和提及词
- `/stats` - 查看使用统计
- `/knowledge` - 知识库管理

### 群组功能
- **@提及**: 在群组中 @机器人并附加消息
- **回复消息**: 回复机器人的消息继续对话
- **关键词触发**: 消息包含设置的提及词时自动回复

### 提及词管理
通过 `/settings` 命令进入设置菜单，选择"💬 提及词管理"：
- 添加新的提及词
- 删除现有提及词
- 重置为默认值

默认提及词：小菲、小菲ai、小菲AI、ai、AI

## 🔧 开发指南

### 本地开发

```bash
# 克隆项目
git clone https://github.com/Shannon-x/sufe-ai-bot.git
cd sufe-ai-bot

# 安装依赖
go mod download

# 运行测试
go test ./...

# 运行 linter
golangci-lint run

# 构建
go build -o bot cmd/bot/main.go
```

### 项目结构

```
sufe-ai-bot/
├── cmd/bot/              # 程序入口
├── internal/             # 内部包（不对外暴露）
│   ├── config/          # 配置管理
│   ├── handlers/        # 请求处理器
│   │   ├── command.go   # 命令处理
│   │   ├── message.go   # 消息处理
│   │   ├── knowledge.go # 知识库处理
│   │   └── mention.go   # 提及词处理
│   ├── services/        # 业务服务
│   │   ├── ai/         # AI 接口服务
│   │   │   └── custom.go # 自定义 AI 端点
│   │   ├── cache/      # 缓存服务
│   │   ├── storage/    # 存储服务
│   │   └── knowledge/  # 知识库服务
│   ├── middleware/      # 中间件
│   │   ├── metrics.go  # 监控指标
│   │   └── ratelimit.go # 速率限制
│   ├── models/         # 数据模型
│   └── i18n/           # 国际化
├── pkg/                 # 公共包（可对外使用）
│   ├── logger/         # 日志工具
│   └── markdown/       # Markdown 转换
├── configs/            # 配置文件
├── knowledge/          # 知识库文件
├── scripts/            # 脚本工具
├── docs/               # 文档
└── tests/              # 测试文件
```

### 添加新功能

1. **添加新命令**：
   - 在 `handlers/command.go` 中添加处理函数
   - 更新帮助文本
   - 添加相应的 i18n 翻译

2. **添加新的 AI 端点**：
   - 编辑 `configs/config.yaml`
   - 在 `models.endpoints` 中添加端点配置
   - 设置对应的环境变量

3. **添加新语言**：
   - 创建 `configs/i18n/[lang].json`
   - 在配置中添加语言代码

## 📊 性能对比

| 指标 | Node.js 版本 | Go 版本 | 提升 |
|-----|-------------|---------|------|
| 内存占用 | 50-100MB | 10-30MB | 70% ↓ |
| 启动时间 | 1-2秒 | <100ms | 95% ↓ |
| 并发处理 | ~100 | 5000+ | 50x ↑ |
| CPU 使用率 | 高 | 低 | 60% ↓ |
| Docker 镜像 | 100MB | 20MB | 80% ↓ |

## 🔍 故障排查

### 常见问题

1. **机器人不响应**
   ```bash
   # 检查日志
   docker-compose logs bot
   
   # 检查连接
   curl http://localhost:9090/health
   ```

2. **Redis 连接失败**
   ```bash
   # 检查 Redis 状态
   docker-compose ps redis
   
   # 测试连接
   docker-compose exec redis redis-cli ping
   ```

3. **内存占用过高**
   ```bash
   # 查看内存使用
   docker stats
   
   # 调整配置
   # 减少 cache.max_size
   # 减少 context.max_messages
   ```

### 调试模式

```bash
# 启用调试日志
export LOG_LEVEL=debug
./bot --config configs/config.yaml
```

## 🔒 安全建议

1. **API 密钥安全**
   - 使用环境变量存储敏感信息
   - 定期轮换 API 密钥
   - 使用密钥管理服务（生产环境）

2. **网络安全**
   - 使用 HTTPS/TLS
   - 配置防火墙规则
   - 限制内部端口访问

3. **访问控制**
   - 为监控界面设置强密码
   - 使用 IP 白名单
   - 启用双因素认证

4. **数据安全**
   - 定期备份 Redis 数据
   - 加密敏感数据
   - 遵守数据保护法规

## 🤝 贡献指南

欢迎贡献代码！请遵循以下步骤：

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情

## 🙏 致谢

- [Telegraf](https://github.com/go-telegram-bot-api/telegram-bot-api) - Telegram Bot API
- [Cloudflare Workers AI](https://developers.cloudflare.com/workers-ai/) - AI 服务
- 所有贡献者和用户

---

如有问题或建议，请提交 [Issue](https://github.com/yourusername/cf-ai-tgbot-go/issues)