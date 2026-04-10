package stream

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"time"

	"github.com/puper/apiretry/internal/util"
)

// StreamProbe 探测 SSE 流的首包，验证后返回事件和剩余流。
type StreamProbe interface {
	ProbeFirstEvent(ctx context.Context, body io.ReadCloser, timeout time.Duration) (preRead []byte, rest io.ReadCloser, event *SSEEvent, err error)
}

// DefaultProbe 是默认的 StreamProbe 实现。
type DefaultProbe struct{}

// ProbeFirstEvent 等待并验证第一个 SSE 事件。
// 成功时返回 preRead（预读字节）、rest（组合了预读+原始流的 ReadCloser）和事件。
// 失败时关闭 body 以释放资源，返回错误。
func (p *DefaultProbe) ProbeFirstEvent(ctx context.Context, body io.ReadCloser, timeout time.Duration) (preRead []byte, rest io.ReadCloser, event *SSEEvent, err error) {
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	br := bufio.NewReaderSize(body, 4096)

	var lines []string

	for {
		type readResult struct {
			line string
			err  error
		}

		ch := make(chan readResult, 1)
		go func() {
			line, err := br.ReadString('\n')
			ch <- readResult{line: line, err: err}
		}()

		select {
		case <-probeCtx.Done():
			body.Close()
			<-ch
			if ctx.Err() != nil {
				return nil, nil, nil, ctx.Err()
			}
			return nil, nil, nil, &util.FirstByteTimeoutError{Timeout: timeout}
		case result := <-ch:
			line := result.line
			readErr := result.err

			if readErr != nil {
				if readErr == io.EOF && line == "" {
					util.DrainBody(body, 1<<20)
					if probeCtx.Err() != nil {
						if ctx.Err() != nil {
							return nil, nil, nil, ctx.Err()
						}
						return nil, nil, nil, &util.FirstByteTimeoutError{Timeout: timeout}
					}
					return nil, nil, nil, io.ErrUnexpectedEOF
				}
				if readErr != io.EOF {
					util.DrainBody(body, 1<<20)
					return nil, nil, nil, readErr
				}
			}

			line = trimLine(line)
			if line == "" {
				if len(lines) > 0 {
					sseEvent, decodeErr := DecodeEvent(lines)
					if decodeErr != nil {
						util.DrainBody(body, 1<<20)
						return nil, nil, nil, decodeErr
					}
					if validateErr := ValidateFirstEvent(sseEvent.Data); validateErr != nil {
						util.DrainBody(body, 1<<20)
						return nil, nil, nil, validateErr
					}

					buffered := br.Buffered()
					peek, peekErr := br.Peek(buffered)
					if peekErr != nil {
						util.DrainBody(body, 1<<20)
						return nil, nil, nil, peekErr
					}

					rest = io.NopCloser(io.MultiReader(bytes.NewReader(peek), br))
					return peek, rest, sseEvent, nil
				}
				continue
			}

			lines = append(lines, line)
		}
	}
}

func trimLine(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
