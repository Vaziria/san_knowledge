package san_exness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// Stream types (Message.Type): the API's event_type values, plus EventTick
// for the ticks stream and EventError for a refused command.
const (
	EventSnapshot     = "trading_state_snapshot"
	EventTransaction  = "transaction_event"
	EventAccountState = "account_state_event"
	EventInstrument   = "instrument_event"
	EventHMRSnapshot  = "hmr_snapshot"
	EventHMRUpdate    = "hmr_update"
	EventTick         = "tick"
	EventError        = "error"
)

// Subprotocol is what Exness's own WebSocket example asks for.
const Subprotocol = "exness-ws-protocol"

// Subscription is what a stream subscribes to.
type Subscription struct {
	Event       string   `json:"event"`
	Instruments []string `json:"instruments,omitempty"`
}

// Transactions: a trading_state_snapshot, then every transaction_event
// (the results of trading requests, balance operations).
func Transactions() Subscription { return Subscription{Event: "transactions"} }

// AccountStateUpdates: balance, equity and used margin every 2 seconds.
func AccountStateUpdates() Subscription { return Subscription{Event: "account_state"} }

// InstrumentUpdates: condition changes of these instruments (none: all).
func InstrumentUpdates(names ...string) Subscription {
	if len(names) == 0 {
		names = []string{"all"}
	}
	return Subscription{Event: "instruments", Instruments: names}
}

// HMR: high margin requirement periods of these instruments.
func HMR(names ...string) Subscription { return Subscription{Event: "hmr", Instruments: names} }

// TicksOf: live bid and ask of these instruments (the ticks stream only).
func TicksOf(names ...string) Subscription { return Subscription{Event: "ticks", Instruments: names} }

type command struct {
	ID          string        `json:"id"`
	Subscribe   *Subscription `json:"subscribe,omitempty"`
	Unsubscribe *Subscription `json:"unsubscribe,omitempty"`
}

// Stream is one WebSocket connection: Events or Ticks. It gives no replay
// after a drop: on an error, dial again, subscribe again, and rebuild
// state from the new snapshot.
type Stream struct {
	conn   *websocket.Conn
	events bool
}

// Events opens the server events stream (transactions, account state,
// instruments, HMR).
func (c *Client) Events(ctx context.Context) (*Stream, error) {
	return c.dial(ctx, "/ws/events", true)
}

// Ticks opens the live ticks stream.
func (c *Client) Ticks(ctx context.Context) (*Stream, error) {
	return c.dial(ctx, "/ws/ticks", false)
}

func (c *Client) dial(ctx context.Context, tail string, events bool) (*Stream, error) {
	ap, err := c.AccessPoint(ctx)
	if err != nil {
		return nil, err
	}
	path := c.accountPath("server-events", tail)
	hc := *c.http
	hc.Timeout = 0 // coder/websocket wants the context to time the handshake
	for attempt := 0; ; attempt++ {
		h, err := c.signer.Sign("GET", path, "", nil, c.clock())
		if err != nil {
			return nil, err
		}
		header := http.Header{"User-Agent": {c.agent}}
		for name, value := range h.Map() {
			header[name] = []string{value}
		}
		dctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		sent := c.now()
		conn, resp, err := websocket.Dial(dctx, "wss://"+strings.TrimPrefix(ap, "https://")+path, &websocket.DialOptions{
			HTTPClient:   &hc,
			HTTPHeader:   header,
			Subprotocols: []string{Subprotocol},
		})
		cancel()
		if err == nil {
			conn.SetReadLimit(16 << 20) // snapshots and instrument lists exceed the 32 KiB default
			return &Stream{conn: conn, events: events}, nil
		}
		if resp == nil {
			return nil, fmt.Errorf("exness: stream %s: %w", tail, err)
		}
		skewed := c.learnClock(resp, sent, c.now())
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		if attempt == 0 && skewed && resp.StatusCode == http.StatusUnauthorized {
			continue
		}
		return nil, parseAPIError(resp, data)
	}
}

func (s *Stream) check(sub Subscription) error {
	ticks := sub.Event == "ticks"
	if s.events == ticks {
		if ticks {
			return invalid("ticks have their own stream (Ticks)")
		}
		return invalid("the ticks stream takes only ticks")
	}
	switch sub.Event {
	case "transactions", "account_state":
	case "ticks", "instruments", "hmr":
		if len(sub.Instruments) == 0 {
			return invalid("%s needs instruments", sub.Event)
		}
		for _, n := range sub.Instruments {
			if err := checkInstrument(n); err != nil {
				return err
			}
		}
	default:
		return invalid("subscription %q", sub.Event)
	}
	return nil
}

// Subscribe sends a subscription; a refusal comes back as an EventError
// message with this id.
func (s *Stream) Subscribe(ctx context.Context, id string, sub Subscription) error {
	if err := s.check(sub); err != nil {
		return err
	}
	return s.send(ctx, command{ID: id, Subscribe: &sub})
}

// Unsubscribe ends a subscription.
func (s *Stream) Unsubscribe(ctx context.Context, id string, sub Subscription) error {
	if err := s.check(sub); err != nil {
		return err
	}
	return s.send(ctx, command{ID: id, Unsubscribe: &sub})
}

func (s *Stream) send(ctx context.Context, cmd command) error {
	if cmd.ID == "" {
		return invalid("a command needs an id")
	}
	b, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	return s.conn.Write(ctx, websocket.MessageText, b)
}

// Close ends the stream.
func (s *Stream) Close() error { return s.conn.Close(websocket.StatusNormalClosure, "") }

// Message is one stream message, by Type; Raw is always set.
type Message struct {
	Type         string
	Raw          json.RawMessage
	Snapshot     *Snapshot
	Transaction  *TransactionEvent
	AccountState *AccountStateEvent
	Instrument   *InstrumentEvent
	HMR          *HMREvent // hmr_snapshot (Periods) and hmr_update
	Tick         *Tick
	Error        *StreamError
}

// TransactionEvent is one finalized change, or a trading request's
// rejection. Its payload holds only what this transaction changed.
type TransactionEvent struct {
	OperationID     string    `json:"operation_id,omitempty"`
	ClientRequestID string    `json:"client_request_id,omitempty"`
	Status          string    `json:"status"` // success, failed, rejected
	ErrorCode       int       `json:"error_code,omitempty"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	EventTime       time.Time `json:"event_time,omitzero"`
	Payload         struct {
		Orders       []Order       `json:"orders"`
		Positions    []Position    `json:"positions"`
		Deals        []Deal        `json:"deals"`
		AccountState *AccountState `json:"account_state"`
	} `json:"payload"`
}

// Err is the event as an *OperationError when it was rejected or failed.
func (e *TransactionEvent) Err() error {
	if e.Status == "rejected" || e.Status == "failed" {
		return &OperationError{OperationID: e.OperationID, Status: e.Status, Code: e.ErrorCode, Message: e.ErrorMessage}
	}
	return nil
}

type AccountStateEvent struct {
	EventTime time.Time
	State     AccountState
}

type InstrumentEvent struct {
	EventTime time.Time
	Action    string // new, upd, del
	Condition InstrumentCondition
}

type HMRPeriod struct {
	PeriodID    string   `json:"period_id"`
	EventType   string   `json:"event_type"` // regular, periodic
	Instruments []string `json:"instruments"`
	Leverage    string   `json:"leverage"`
	StartTime   string   `json:"start_time"`
	EndTime     string   `json:"end_time"`
}

type HMREvent struct {
	Periods  []HMRPeriod `json:"periods"`
	Upserted []HMRPeriod `json:"upserted_periods"`
	Removed  []string    `json:"removed_period_ids"`
}

type Tick struct {
	Instrument string    `json:"instrument"`
	Bid        float64   `json:"bid"`
	Ask        float64   `json:"ask"`
	Timestamp  time.Time `json:"timestamp"`
}

// StreamError is a refused stream command.
type StreamError struct {
	ID      string `json:"id"`
	Code    int    `json:"code"`
	Message string `json:"error_message"`
}

func (e *StreamError) Error() string {
	return fmt.Sprintf("exness: stream command %s: %s (%d)", e.ID, e.Message, e.Code)
}

// Next waits for the next message. An unknown event type comes with Raw
// only; a broken connection is an error, after which the stream is done.
func (s *Stream) Next(ctx context.Context) (Message, error) {
	_, data, err := s.conn.Read(ctx)
	if err != nil {
		return Message{}, err
	}
	m := Message{Raw: data}
	var head struct {
		EventType string          `json:"event_type"`
		EventTime time.Time       `json:"event_time"`
		Action    string          `json:"action"`
		Payload   json.RawMessage `json:"payload"`
		Code      *int            `json:"code"`
		ID        string          `json:"id"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return m, fmt.Errorf("exness: stream message: %w", err)
	}
	m.Type = head.EventType
	switch {
	case head.EventType == "" && head.Code != nil:
		m.Type, m.Error = EventError, &StreamError{}
		err = json.Unmarshal(data, m.Error)
	case head.EventType == "" && !s.events:
		m.Type, m.Tick = EventTick, &Tick{}
		err = json.Unmarshal(data, m.Tick)
	case head.EventType == EventSnapshot:
		m.Snapshot = &Snapshot{}
		err = json.Unmarshal(head.Payload, m.Snapshot)
	case head.EventType == EventTransaction:
		m.Transaction = &TransactionEvent{}
		err = json.Unmarshal(data, m.Transaction)
	case head.EventType == EventAccountState:
		var p struct {
			AccountState AccountState `json:"account_state"`
		}
		err = json.Unmarshal(head.Payload, &p)
		m.AccountState = &AccountStateEvent{EventTime: head.EventTime, State: p.AccountState}
	case head.EventType == EventInstrument:
		m.Instrument = &InstrumentEvent{EventTime: head.EventTime, Action: head.Action}
		err = json.Unmarshal(head.Payload, &m.Instrument.Condition)
	case head.EventType == EventHMRSnapshot || head.EventType == EventHMRUpdate:
		m.HMR = &HMREvent{}
		err = json.Unmarshal(data, m.HMR)
	}
	if err != nil {
		return m, fmt.Errorf("exness: stream %s: %w", m.Type, err)
	}
	return m, nil
}
