package requests

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// pageSize caps every list; paging comes when lists grow.
const pageSize = 50

// PostgresStore keeps repair requests and responses, through GORM. The
// tables are made by the migrations in internal/db.
type PostgresStore struct {
	db *gorm.DB
}

func NewPostgresStore(gdb *gorm.DB) *PostgresStore {
	return &PostgresStore{db: gdb}
}

// requestRow is a request as the lists read it.
type requestRow struct {
	ID            int64
	Service       string
	Area          string
	Description   string
	CreatedAt     time.Time
	CustomerName  string
	CustomerPhone string
	Responded     bool
}

func (r requestRow) request() Request {
	return Request{
		ID: r.ID, Service: r.Service, Area: r.Area, Description: r.Description, CreatedAt: r.CreatedAt,
		CustomerName: r.CustomerName, CustomerPhone: r.CustomerPhone, Responded: r.Responded,
	}
}

func (s *PostgresStore) Create(ctx context.Context, customerID int64, in NewRequest) (Request, error) {
	var row requestRow
	err := s.db.WithContext(ctx).Raw(`
		WITH inserted AS (
			INSERT INTO repair_requests (customer_id, service, area, description)
			VALUES (?, ?, ?, ?)
			RETURNING id, customer_id, service, area, description, created_at
		)
		SELECT i.id, i.service, i.area, i.description, i.created_at, a.full_name AS customer_name
		FROM inserted i JOIN accounts a ON a.id = i.customer_id
	`, customerID, in.Service, in.Area, in.Description).Scan(&row).Error
	if err != nil {
		return Request{}, fmt.Errorf("insert request: %w", err)
	}
	return row.request(), nil
}

func (s *PostgresStore) ByCustomer(ctx context.Context, customerID int64) ([]Request, error) {
	var rows []requestRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT r.id, r.service, r.area, r.description, r.created_at, a.full_name AS customer_name
		FROM repair_requests r JOIN accounts a ON a.id = r.customer_id
		WHERE r.customer_id = ?
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT ?
	`, customerID, pageSize).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}
	list := make([]Request, len(rows))
	if len(rows) == 0 {
		return list, nil
	}
	index := map[int64]int{}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		list[i] = r.request()
		list[i].Responses = []Response{}
		ids[i] = r.ID
		index[r.ID] = i
	}
	var responses []struct {
		RequestID int64
		Response
	}
	err = s.db.WithContext(ctx).Raw(`
		SELECT resp.request_id, resp.id, resp.repairman_id, a.full_name AS repairman_name, a.phone AS repairman_phone,
		       resp.message, resp.created_at
		FROM request_responses resp JOIN accounts a ON a.id = resp.repairman_id
		WHERE resp.request_id IN ?
		ORDER BY resp.created_at, resp.id
	`, ids).Scan(&responses).Error
	if err != nil {
		return nil, fmt.Errorf("list responses: %w", err)
	}
	for _, r := range responses {
		i := index[r.RequestID]
		list[i].Responses = append(list[i].Responses, r.Response)
		list[i].Responded = true
	}
	return list, nil
}

func (s *PostgresStore) Open(ctx context.Context, repairmanID int64, service string) ([]Request, error) {
	var rows []requestRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT r.id, r.service, r.area, r.description, r.created_at, c.full_name AS customer_name,
		       resp.id IS NOT NULL AS responded,
		       CASE WHEN resp.id IS NOT NULL THEN c.phone ELSE '' END AS customer_phone
		FROM repair_requests r
		JOIN accounts c ON c.id = r.customer_id
		LEFT JOIN request_responses resp ON resp.request_id = r.id AND resp.repairman_id = @repairman
		WHERE (CAST(@service AS text) = '' OR r.service = CAST(@service AS text))
		  -- A confirmed or done visit takes the request off the open list;
		  -- a cancelled one or a no-show puts it back.
		  AND NOT EXISTS (SELECT 1 FROM visits v WHERE v.request_id = r.id AND v.status IN ('confirmed', 'arrived'))
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT @limit
	`, map[string]any{"repairman": repairmanID, "service": service, "limit": pageSize}).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list open requests: %w", err)
	}
	list := make([]Request, len(rows))
	for i, r := range rows {
		list[i] = r.request()
	}
	return list, nil
}

// responseRow is a row of request_responses.
type responseRow struct {
	ID          int64 `gorm:"primaryKey"`
	RequestID   int64
	RepairmanID int64
	Message     string
	CreatedAt   time.Time
}

func (responseRow) TableName() string { return "request_responses" }

func (s *PostgresStore) Respond(ctx context.Context, requestID, repairmanID int64, message string) (Response, error) {
	resp := Response{RepairmanID: repairmanID, Message: message}
	row := responseRow{RequestID: requestID, RepairmanID: repairmanID, Message: message}
	// Create, not Raw: GORM translates its errors (Raw(...).Scan's it does not).
	err := s.db.WithContext(ctx).Create(&row).Error
	switch {
	case errors.Is(err, gorm.ErrForeignKeyViolated): // no such request
		return Response{}, ErrNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return Response{}, ErrAlreadyResponded
	case err != nil:
		return Response{}, fmt.Errorf("insert response: %w", err)
	}
	resp.ID, resp.CreatedAt = row.ID, row.CreatedAt
	return resp, nil
}
