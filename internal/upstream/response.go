package upstream

import (
	"net/http"
	"time"
)

// ResponseInfo 记录上游响应的摘要信息
type ResponseInfo struct {
	StatusCode int
	Duration   time.Duration
}

// ExtractInfo 从 HTTP 响应和起始时间提取响应信息
func ExtractInfo(resp *http.Response, start time.Time) ResponseInfo {
	return ResponseInfo{
		StatusCode: resp.StatusCode,
		Duration:   time.Since(start),
	}
}
