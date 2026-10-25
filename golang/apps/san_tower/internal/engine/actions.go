package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

// HTTPTimeout is the longest an HTTP Request step waits for its answer.
const HTTPTimeout = 30 * time.Second

// maxResponse is the most of an answer's body an HTTP Request step reads.
const maxResponse = 10 << 20

// httpRequest sends Body (when linked) as JSON with Method to URL, whose
// {{ expressions }} read Body as payload. Response is the answer: status,
// headers (the first value of each) and body, as JSON when it is JSON, else
// as text. A 4xx or 5xx answer fails the step, as in n8n; its Response is
// still in the run.
func httpRequest(c *call) error {
	method := strings.ToUpper(c.setting("Method"))
	if method == "" {
		method = http.MethodPost
	}
	body, linked := c.inputs["Body"].(*structpb.Value)
	raw, err := interpolate(c.setting("URL"), c.object("Body"))
	if err != nil {
		return fmt.Errorf("URL: %w", err)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("URL %w", errEmpty)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("URL %q is not an http or https address", raw)
	}

	var rd io.Reader
	if linked && method != http.MethodGet {
		b, err := json.Marshal(body.AsInterface())
		if err != nil {
			return fmt.Errorf("Body: %w", err)
		}
		rd = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(c.ctx, HTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return err
	}
	if rd != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json, */*")
	req.Header.Set("User-Agent", "san_tower")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) && c.ctx.Err() == nil {
			return fmt.Errorf("%s %s: no answer in %s", method, u, HTTPTimeout)
		}
		return fmt.Errorf("%s %s: %w", method, u, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return fmt.Errorf("%s %s: reading the answer: %w", method, u, err)
	}

	headers := map[string]any{}
	for k, v := range resp.Header {
		headers[strings.ToLower(k)] = v[0]
	}
	var answer any = string(data)
	if strings.Contains(resp.Header.Get("Content-Type"), "json") {
		var j any
		if json.Unmarshal(data, &j) == nil {
			answer = j
		}
	}
	out, err := structpb.NewValue(map[string]any{"status": resp.StatusCode, "headers": headers, "body": answer})
	if err != nil {
		return err
	}
	c.outputs["Response"] = out
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s: %s: %s", method, u, resp.Status, snippet(data))
	}
	c.fire("Done")
	return nil
}

// snippet is the start of an answer's body, for an error.
func snippet(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// publish publishes Payload on Topic in the tower's broker: an object or a
// list as JSON, text as it is, nothing when Payload is not linked. Result is
// the topic and the event's id.
func publish(c *call) error {
	topic := c.setting("Topic")
	if topic == "" {
		return fmt.Errorf("Topic %w", errEmpty)
	}
	if c.env.Publish == nil {
		return errors.New("this run cannot publish")
	}
	var data []byte
	if p, ok := c.inputs["Payload"].(*structpb.Value); ok {
		if s, isText := p.Kind.(*structpb.Value_StringValue); isText {
			data = []byte(s.StringValue)
		} else {
			b, err := json.Marshal(p.AsInterface())
			if err != nil {
				return fmt.Errorf("Payload: %w", err)
			}
			data = b
		}
	}
	id, err := c.env.Publish(c.ctx, topic, data)
	if err != nil {
		return err
	}
	out, err := structpb.NewValue(map[string]any{"topic": topic, "eventId": fmt.Sprint(id)})
	if err != nil {
		return err
	}
	c.outputs["Result"] = out
	c.fire("Done")
	return nil
}
