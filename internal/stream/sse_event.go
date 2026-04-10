package stream

// SSEEvent 表示一个 Server-Sent Events 事件。
// 规范参考：https://html.spec.whatwg.org/multipage/server-sent-events.html
type SSEEvent struct {
	ID        string // 事件 ID，来自 "id:" 字段
	Data      string // 事件数据，来自 "data:" 字段（多行 data 会用 \n 连接）
	EventType string // 事件类型，来自 "event:" 字段
	Retry     int    // 重连间隔（毫秒），来自 "retry:" 字段；0 表示未设置
}
