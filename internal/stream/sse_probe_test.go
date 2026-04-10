package stream

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/puper/apiretry/internal/util"
)

func buildSSEStream(events []string) io.ReadCloser {
	var buf bytes.Buffer
	for _, evt := range events {
		buf.WriteString(evt)
		buf.WriteString("\n")
	}
	return io.NopCloser(&buf)
}

func TestProbeFirstEvent_ValidFirstEvent(t *testing.T) {
	firstEvent := "data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\nid: chatcmpl-1\n\n"
	secondEvent := "data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"delta\":{\"content\":\" there\"}}]}\nid: chatcmpl-1\n\n"
	fullStream := firstEvent + secondEvent + "data: [DONE]\n\n"
	body := io.NopCloser(bytes.NewReader([]byte(fullStream)))

	probe := &DefaultProbe{}
	preRead, rest, event, err := probe.ProbeFirstEvent(context.Background(), body, 5*time.Second)
	if err != nil {
		t.Fatalf("ProbeFirstEvent() error = %v", err)
	}
	if event.ID != "chatcmpl-1" {
		t.Errorf("event.ID = %q, want %q", event.ID, "chatcmpl-1")
	}
	if len(preRead) == 0 {
		t.Error("preRead should not be empty")
	}
	restData, readErr := io.ReadAll(rest)
	if readErr != nil {
		t.Fatalf("ReadAll rest error = %v", readErr)
	}
	_ = restData
}

func TestProbeFirstEvent_InvalidJSON(t *testing.T) {
	stream := "data: {invalid json}\nid: 1\n\n"
	body := io.NopCloser(bytes.NewReader([]byte(stream)))

	probe := &DefaultProbe{}
	_, _, _, err := probe.ProbeFirstEvent(context.Background(), body, 5*time.Second)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestProbeFirstEvent_MissingRequiredFields(t *testing.T) {
	stream := "data: {\"model\":\"gpt-4\",\"object\":\"list\"}\nid: 1\n\n"
	body := io.NopCloser(bytes.NewReader([]byte(stream)))

	probe := &DefaultProbe{}
	_, _, _, err := probe.ProbeFirstEvent(context.Background(), body, 5*time.Second)
	if err == nil {
		t.Fatal("expected error for missing id/choices fields, got nil")
	}
}

func TestProbeFirstEvent_Timeout(t *testing.T) {
	r, _ := io.Pipe()
	defer r.Close()

	probe := &DefaultProbe{}
	_, _, _, err := probe.ProbeFirstEvent(context.Background(), r, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if _, ok := err.(*util.FirstByteTimeoutError); !ok {
		t.Errorf("expected FirstByteTimeoutError, got %T: %v", err, err)
	}
}

func TestProbeFirstEvent_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	body := io.NopCloser(bytes.NewReader(nil))
	cancel()

	probe := &DefaultProbe{}
	_, _, _, err := probe.ProbeFirstEvent(ctx, body, 5*time.Second)
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %T: %v", err, err)
	}
}

func TestProbeFirstEvent_DataIntegrity(t *testing.T) {
	firstEvent := "data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\nid: 1\n\n"
	secondEvent := "data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"B\"}}]}\nid: 1\n\n"
	doneEvent := "data: [DONE]\n\n"

	fullStream := firstEvent + secondEvent + doneEvent
	originalData := []byte(fullStream)
	body := io.NopCloser(bytes.NewReader(originalData))

	probe := &DefaultProbe{}
	preRead, rest, event, err := probe.ProbeFirstEvent(context.Background(), body, 5*time.Second)
	if err != nil {
		t.Fatalf("ProbeFirstEvent() error = %v", err)
	}
	if event.ID != "1" {
		t.Errorf("event.ID = %q, want %q", event.ID, "1")
	}

	restData, readErr := io.ReadAll(rest)
	if readErr != nil {
		t.Fatalf("ReadAll rest error = %v", readErr)
	}
	rest.Close()

	allData := append(preRead, restData...)
	if !bytes.Contains(allData, []byte(`"content":"B"`)) {
		t.Error("combined data missing second event content")
	}
	if !bytes.Contains(allData, []byte("[DONE]")) {
		t.Error("combined data missing [DONE] signal")
	}
}

func TestProbeFirstEvent_DoneSignal(t *testing.T) {
	stream := "data: [DONE]\n\n"
	body := io.NopCloser(bytes.NewReader([]byte(stream)))

	probe := &DefaultProbe{}
	_, _, event, err := probe.ProbeFirstEvent(context.Background(), body, 5*time.Second)
	if err != nil {
		t.Fatalf("ProbeFirstEvent() error = %v", err)
	}
	if event.Data != "[DONE]" {
		t.Errorf("event.Data = %q, want %q", event.Data, "[DONE]")
	}
}
