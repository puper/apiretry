# 架构设计

## 模块结构

```
cmd/proxy/main.go                    # 入口：加载配置、创建依赖、启动服务
internal/
  config/config.go                    # YAML 配置加载，支持 ${ENV_VAR} 环境变量替换
  server/
    router.go                         # HTTP 路由：/v1/* → proxy, /health, /ready
    middleware.go                      # 中间件：Request-ID, Body-Size-Limit, Logging
    handlers.go                        # Health/Ready 端点
  proxy/
    proxy.go                          # 主入口：路由到流式/非流式代理
    stream_proxy.go                   # 流式首包闸门重试（核心）
    nonstream_proxy.go                # 非流式重试
    attempt.go                        # 重试计数器和预算追踪
  retry/
    classifier.go                     # 错误分类器（429/5xx/网络/首包超时）
    backoff.go                        # 退避策略 + jitter + Retry-After 解析
    policy.go                         # 重试决策组合器 + 预算执行
  upstream/
    client.go                         # http.Client 封装（Doer 接口）
    request.go                        # 上游请求构建：URL 重写、header 处理
    response.go                       # 响应信息提取
  stream/
    sse_probe.go                      # SSE 首包探测器（可取消读取）
    sse_parse.go                      # SSE 行解析 + L2 级 JSON 验证
    sse_event.go                      # SSE 事件结构体
    sse_is_stream.go                  # 流式请求检测
    flush.go                          # FlushWriter（ResponseController 降级）
  observe/
    logger.go                         # slog 日志初始化
  util/
    errors.go                         # 错误类型：UpstreamError, FirstByteTimeoutError, BudgetExceededError...
    body.go                           # 请求体缓存和排空
    header.go                         # Hop-by-hop header 处理
    context.go                        # Request-ID/Attempt 上下文值
    response.go                       # OpenAI 兼容错误响应
```

## 核心流程

### 流式首包闸门

```
客户端请求 → 缓存 body → 判断 stream? → 是 → StreamProxy
                                           否 → NonStreamProxy

StreamProxy:
  for attempt := 1; attempt <= maxAttempts:
    构建上游请求 → 发送 → 收到响应
    ├─ 网络错误/可重试状态码 → 分类 → 决策重试
    └─ 200 OK:
         ProbeFirstEvent(ctx, body, timeout)
         ├─ 超时/失败 → 分类 → 决策重试（首包前，可重试）
         └─ 成功 → 提交响应头+首事件 → 转发剩余流 → 结束

  预算耗尽 → 504 proxy_retry_exhausted
```

### 非流式重试

```
NonStreamProxy:
  for attempt := 1; attempt <= maxAttempts:
    构建上游请求 → 发送 → 读取完整响应
    ├─ 可重试错误 → DrainBody → sleep → 重试
    └─ 成功/不可重试 → 返回响应

  预算耗尽 → 504 proxy_retry_exhausted
```

## 关键接口

```go
type Doer interface { Do(req *http.Request) (*http.Response, error) }
type StreamProbe interface { ProbeFirstEvent(ctx, body, timeout) (preRead, rest, event, err) }
type RetryPolicy interface { Decide(input DecideInput) RetryDecision }
```

## 重试策略

| 错误类型 | 退避 schedule | 最大单次延迟 |
|---|---|---|
| 429 Rate Limit | 200ms, 500ms, 1s, 2s, 3s | 3s |
| 5xx Server Error | 100ms, 300ms, 700ms, 1.5s, 2.5s | 3s |

总重试预算：10秒，最大尝试次数：5次

## SSE 首事件验证（L2）

- 必须包含完整 `data:` 行 + 空行分隔
- `data: [DONE]` 合法
- JSON payload 必须包含 `id` 或 `choices` 字段
- `bufio.Reader.Peek(Buffered())` 取回预读数据，`io.MultiReader` 组合流

## 不可重试状态码

400, 401, 403, 404, 409, 422 → 直接返回上游响应