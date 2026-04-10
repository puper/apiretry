# APIRetry

面向 OpenCode 的 LLM API 首包重试反向代理网关。

## 功能特性

- **流式首包闸门重试**：在 SSE 首事件验证通过前拦截响应，失败可重试
- **非流式完整重试**：对非流式请求的 429/5xx 等可重试状态码自动重试
- **智能退避策略**：支持 429/5xx 分级退避 schedule，带 jitter 抖动
- **重试预算控制**：总重试预算与最大尝试次数双重限制，防止无限重试
- **结构化日志**：所有请求日志携带 `request_id`、`method`、`path`，便于排障
- **配置化驱动**：通过 YAML 配置文件控制上游、重试、限流等行为

## 快速开始

### 构建

```bash
make build
# 输出: bin/apiretry
```

Linux 交叉编译：

```bash
make build-linux
# 输出: bin/apiretry-linux-amd64
```

### 配置

复制示例配置并修改：

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml，设置 upstream.base_url 等参数
```

### 运行

```bash
./bin/apiretry -config config.yaml
```

默认配置文件路径为 `config.yaml`。

## 配置说明

| 配置项 | 说明 |
|---|---|
| `server.addr` | 监听地址，默认 `:8080` |
| `upstream.base_url` | 上游 API 基础 URL |
| `retry.max_attempts` | 最大尝试次数，默认 5 |
| `retry.max_retry_delay_budget` | 总重试预算，默认 10s |
| `retry.first_byte_timeout` | 流式首包超时，默认 8s |
| `retry.chunk_idle_timeout` | 流式分块空闲超时，默认 30s |
| `retry.retry_status_codes` | 可重试 HTTP 状态码列表 |
| `retry.schedule_429` | 429 退避 schedule |
| `retry.schedule_5xx` | 5xx 退避 schedule |
| `limits.max_request_body_bytes` | 请求体大小限制，默认 10MB |
| `logging.level` | 日志级别：debug/info/warn/error |
| `logging.json` | 是否使用 JSON 格式输出 |

完整配置项说明见 [docs/config.md](docs/config.md)。

## 测试

```bash
# 全部测试
make test

# 单个包
go test ./internal/proxy/... -v

# 集成测试
go test ./tests/integration/... -v -timeout 60s
```

## 文档

- [架构设计](docs/architecture.md) — 模块结构、核心流程、接口定义
- [配置参考](docs/config.md) — 完整配置项说明
- [开发备忘](docs/dev_manual.md) — 构建、测试、运行指南

## 许可证

MIT
