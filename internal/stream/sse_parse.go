package stream

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func ParseSSELine(line string) (field, value string) {
	line = strings.TrimRight(line, "\r\n")

	if line == "" {
		return "", ""
	}

	if strings.HasPrefix(line, ":") {
		return "", line
	}

	idx := strings.Index(line, ":")
	if idx < 0 {
		return line, ""
	}

	field = line[:idx]
	value = line[idx+1:]
	if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}
	return field, value
}

// DecodeEvent 将一组 SSE 文本行组装为 SSEEvent。多行 data 用 "\n" 连接。
func DecodeEvent(lines []string) (*SSEEvent, error) {
	var dataParts []string
	event := &SSEEvent{}

	for _, line := range lines {
		field, value := ParseSSELine(line)
		if field == "" {
			continue
		}

		switch field {
		case "data":
			dataParts = append(dataParts, value)
		case "id":
			event.ID = value
		case "event":
			event.EventType = value
		case "retry":
			retry, err := strconv.Atoi(value)
			if err != nil {
				continue
			}
			event.Retry = retry
		}
	}

	event.Data = strings.Join(dataParts, "\n")
	return event, nil
}

// ValidateFirstEvent 对首事件 data 做 L2 验证：JSON 合法性 + 包含 "id" 或 "choices" 字段。
// "data: [DONE]" 是流结束信号，直接通过。
func ValidateFirstEvent(data string) error {
	if data == "[DONE]" {
		return nil
	}

	if !json.Valid([]byte(data)) {
		return fmt.Errorf("first event data is not valid JSON: %s", truncate(data, 100))
	}

	hasID := strings.Contains(data, `"id"`)
	hasChoices := strings.Contains(data, `"choices"`)
	if !hasID && !hasChoices {
		return fmt.Errorf("first event JSON missing required fields (\"id\" or \"choices\"): %s", truncate(data, 100))
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
