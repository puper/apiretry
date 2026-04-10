package stream

import (
	"testing"
)

func TestParseSSELine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantField string
		wantValue string
	}{
		{"data字段", "data: hello", "data", "hello"},
		{"id字段", "id: 123", "id", "123"},
		{"event字段", "event: message", "event", "message"},
		{"retry字段", "retry: 5000", "retry", "5000"},
		{"空格在值中", "data: hello world", "data", "hello world"},
		{"冒号后无空格", "data:hello", "data", "hello"},
		{"多冒号", "data: http://example.com", "data", "http://example.com"},
		{"注释行", ": this is a comment", "", ": this is a comment"},
		{"空行", "", "", ""},
		{"CRLF行", "data: test\r\n", "data", "test"},
		{"无冒号行", "retry", "retry", ""},
		{"冒号后仅空格", "data: ", "data", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, value := ParseSSELine(tt.line)
			if field != tt.wantField {
				t.Errorf("ParseSSELine(%q) field = %q, want %q", tt.line, field, tt.wantField)
			}
			if value != tt.wantValue {
				t.Errorf("ParseSSELine(%q) value = %q, want %q", tt.line, value, tt.wantValue)
			}
		})
	}
}

func TestDecodeEvent(t *testing.T) {
	tests := []struct {
		name    string
		lines   []string
		want    *SSEEvent
		wantErr bool
	}{
		{
			name: "简单事件",
			lines: []string{
				"data: {\"id\":\"chatcmpl-123\",\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}",
				"id: chatcmpl-123",
			},
			want: &SSEEvent{
				ID:   "chatcmpl-123",
				Data: `{"id":"chatcmpl-123","choices":[{"delta":{"content":"Hi"}}]}`,
			},
		},
		{
			name: "多行data",
			lines: []string{
				"data: line1",
				"data: line2",
				"data: line3",
			},
			want: &SSEEvent{
				Data: "line1\nline2\nline3",
			},
		},
		{
			name: "带event类型",
			lines: []string{
				"event: add",
				"data: payload",
				"id: 42",
			},
			want: &SSEEvent{
				ID:        "42",
				Data:      "payload",
				EventType: "add",
			},
		},
		{
			name: "retry字段",
			lines: []string{
				"retry: 3000",
				"data: test",
			},
			want: &SSEEvent{
				Data:  "test",
				Retry: 3000,
			},
		},
		{
			name: "无效retry忽略",
			lines: []string{
				"retry: abc",
				"data: test",
			},
			want: &SSEEvent{
				Data:  "test",
				Retry: 0,
			},
		},
		{
			name: "注释行跳过",
			lines: []string{
				": this is a comment",
				"data: hello",
			},
			want: &SSEEvent{
				Data: "hello",
			},
		},
		{
			name: "DONE事件",
			lines: []string{
				"data: [DONE]",
			},
			want: &SSEEvent{
				Data: "[DONE]",
			},
		},
		{
			name:    "空行列表",
			lines:   []string{},
			want:    &SSEEvent{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeEvent(tt.lines)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.ID != tt.want.ID || got.Data != tt.want.Data || got.EventType != tt.want.EventType || got.Retry != tt.want.Retry {
				t.Errorf("DecodeEvent() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidateFirstEvent(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "合法JSON带id",
			data:    `{"id":"chatcmpl-123","choices":[{"delta":{"content":"Hi"}}]}`,
			wantErr: false,
		},
		{
			name:    "合法JSON带choices",
			data:    `{"choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
			wantErr: false,
		},
		{
			name:    "DONE信号通过",
			data:    "[DONE]",
			wantErr: false,
		},
		{
			name:    "无效JSON",
			data:    `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "合法JSON但缺少id和choices",
			data:    `{"model":"gpt-4","object":"list"}`,
			wantErr: true,
		},
		{
			name:    "空字符串",
			data:    "",
			wantErr: true,
		},
		{
			name:    "合法JSON带id和choices",
			data:    `{"id":"chatcmpl-123","choices":[{"index":0}]}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFirstEvent(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFirstEvent(%q) error = %v, wantErr %v", tt.data, err, tt.wantErr)
			}
		})
	}
}
