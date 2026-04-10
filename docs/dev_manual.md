# 开发备忘

## 构建

```bash
make build
# 输出: bin/apiretry
```

## 测试

```bash
# 全部测试
make test

# 单个包
go test ./internal/proxy/... -v

# 集成测试
go test ./tests/integration/... -v -timeout 60s
```

## 运行

```bash
# 使用配置文件
./bin/apiretry -config config.yaml

# 默认配置文件路径: config.yaml
```

## 关键设计决策

1. **不使用 httputil.ReverseProxy** — 需要在首包前拦截响应，标准库的 ReverseProxy 会立即提交响应头
2. **bufio.Reader + Peek/MultiReader** — 探测 SSE 首事件后取回预读数据
3. **goroutine + channel 实现可取消读取** — `br.ReadString('\n')` 是阻塞调用，用 select + context 实现超时
4. **http.Client.Timeout = 0** — 流式请求不能用全局超时，通过 firstByteTimeout 和 chunkIdleTimeout 分别控制
5. **DrainBody 失败时 close** — 超时/取消场景下直接 Close 而非 Drain（因为 Drain 也会阻塞）

## 项目结构

```
cmd/proxy/main.go      # 入口
internal/config/        # 配置
internal/server/        # HTTP 路由和中间件
internal/proxy/         # 核心代理逻辑
internal/retry/         # 重试策略
internal/upstream/      # 上游客户端
internal/stream/        # SSE 解析和探测
internal/observe/       # 日志
internal/util/          # 工具函数
tests/integration/      # 端到端测试
```