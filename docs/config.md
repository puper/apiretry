# 配置参考

完整配置示例见 `config.example.yaml`。

## 配置项说明

### server

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| addr | string | `:8080` | 监听地址 |
| read_timeout | duration | `15s` | 读超时 |
| write_timeout | duration | `0` | 写超时（0=无限，适合流式） |
| idle_timeout | duration | `60s` | 空闲连接超时 |

### upstream

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| base_url | string | 必填 | 上游 API 地址（如 `https://api.openai.com`） |
| timeout | duration | `120s` | 整体请求超时 |
| response_header_timeout | duration | `20s` | 等待响应头超时 |
| tls_handshake_timeout | duration | `10s` | TLS 握手超时 |
| idle_conn_timeout | duration | `90s` | 空闲连接超时 |
| max_idle_conns | int | `200` | 最大空闲连接数 |
| max_idle_conns_per_host | int | `100` | 每主机最大空闲连接数 |
| force_attempt_http2 | bool | `true` | 尝试 HTTP/2 |

### retry

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| max_attempts | int | `5` | 最大重试次数 |
| max_retry_delay_budget | duration | `10s` | 总退避等待预算 |
| first_byte_timeout | duration | `8s` | 首包等待超时（流式） |
| chunk_idle_timeout | duration | `30s` | 流中 chunk 空闲超时 |
| max_per_retry_delay | duration | `3s` | 单次最大退避延迟 |
| retry_status_codes | []int | `[429,500,502,503,504]` | 可重试状态码 |
| schedule_429 | []duration | `[200ms,500ms,1s,2s,3s]` | 429 退避序列 |
| schedule_5xx | []duration | `[100ms,300ms,700ms,1.5s,2.5s]` | 5xx 退避序列 |
| jitter_percent | float | `0.15` | 抖动百分比（0-1） |

### limits

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| max_request_body_bytes | int64 | `10485760` (10MB) | 请求体大小限制 |

### logging

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| level | string | `info` | 日志级别：debug/info/warn/error |
| json | bool | `true` | JSON 格式日志 |

## 环境变量

配置文件支持 `${ENV_VAR}` 语法，在加载时替换为 `os.Getenv("ENV_VAR")` 的值。如果环境变量未设置，保持原样。