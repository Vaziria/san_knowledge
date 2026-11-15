package san_exness

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Exness's live events stream sent a refused command's code as text, which
// broke the stream (the tower's Live Trading monitor, 2026-11-15).
func TestStreamErrorCodeNumberOrText(t *testing.T) {
	var a StreamError
	if err := json.Unmarshal([]byte(`{"id":"1","code":400,"error_message":"bad"}`), &a); err != nil || a.Code != 400 || a.CodeText != "" {
		t.Fatalf("number code: %+v %v", a, err)
	}
	var b StreamError
	if err := json.Unmarshal([]byte(`{"id":"2","code":"INVALID_REQUEST","error_message":"nope"}`), &b); err != nil || b.CodeText != "INVALID_REQUEST" || b.Message != "nope" {
		t.Fatalf("text code: %+v %v", b, err)
	}
	if !strings.Contains(b.Error(), "INVALID_REQUEST") {
		t.Fatalf("error text: %s", b.Error())
	}
}

// The live events stream sends times as "...+0000" (no colon).
func TestFixZoneOffsets(t *testing.T) {
	in := []byte(`{"event_time":"2026-11-15T16:33:13.04179508+0000","x":"12:00:00+0700 text","y":"2026-11-15T16:33:13Z"}`)
	var v struct {
		EventTime time.Time `json:"event_time"`
		Y         time.Time `json:"y"`
	}
	if err := json.Unmarshal(fixZoneOffsets(in), &v); err != nil {
		t.Fatal(err)
	}
	if v.EventTime.UTC().Format(time.RFC3339Nano) != "2026-11-15T16:33:13.04179508Z" {
		t.Fatalf("event_time %v", v.EventTime)
	}
	if !strings.Contains(string(fixZoneOffsets(in)), `"12:00:00+0700 text"`) {
		t.Fatal("a non-time text was changed")
	}
}

// A subscription's success answer is {"id":..,"code":"200"}: an ack, not
// a refusal (the tower showed "Exness refused a subscription:" for it).
func TestAckIsNotAnError(t *testing.T) {
	var e StreamError
	if err := json.Unmarshal([]byte(`{"id":"san-1","code":"200"}`), &e); err != nil || e.CodeText != "200" {
		t.Fatalf("%+v %v", e, err)
	}
}
