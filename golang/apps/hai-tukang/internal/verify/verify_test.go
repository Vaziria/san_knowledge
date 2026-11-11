package verify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/wargasipil/hai-tukang/internal/db"
)

type sentCode struct{ phone, code string }

type fakeSender struct {
	mu   sync.Mutex
	sent []sentCode
	err  error
}

func (f *fakeSender) SendCode(_ context.Context, phone, code string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, sentCode{phone, code})
	return nil
}

func (f *fakeSender) last(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		t.Fatal("no code sent")
	}
	return f.sent[len(f.sent)-1].code
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }
func wrong(code string) string           { return fmt.Sprintf("%06d", (atoi(code)+1)%1_000_000) }
func atoi(s string) (n int)              { fmt.Sscan(s, &n); return n }
func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	return c
}
func phoneFor(t *testing.T, n int) string {
	return fmt.Sprintf("+62899%02d%06d", n, time.Now().UnixNano()%1_000_000)
}

// newTest returns a Service on the test database with a fake sender and a
// clock the test moves, and a phone number of its own.
func newTest(t *testing.T, n int) (*Service, *fakeSender, *clock, string) {
	t.Helper()
	dsn := os.Getenv("HAI_TUKANG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set HAI_TUKANG_TEST_DATABASE_URL to a disposable Hai Tukang test database")
	}
	pool, err := db.Open(ctx(t), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(pool) })
	if err := db.Migrate(ctx(t), pool); err != nil {
		t.Fatal(err)
	}
	phone := phoneFor(t, n)
	t.Cleanup(func() { pool.WithContext(context.Background()).Exec(`DELETE FROM phone_codes WHERE phone = ?`, phone) })
	sender := &fakeSender{}
	c := &clock{t: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)}
	s := NewService(NewPostgresStore(pool), sender)
	s.now = c.now
	return s, sender, c, phone
}

func TestSendGivesASixDigitCodeThatWorksOnce(t *testing.T) {
	s, sender, _, phone := newTest(t, 1)
	if err := s.Send(ctx(t), phone, PurposeSignUp); err != nil {
		t.Fatal(err)
	}
	code := sender.last(t)
	if !regexp.MustCompile(`^\d{6}$`).MatchString(code) {
		t.Fatalf("code = %q, want 6 digits", code)
	}
	if err := s.Check(ctx(t), phone, PurposeSignUp, code); err != nil {
		t.Fatalf("Check(right code) = %v, want nil", err)
	}
	if err := s.Check(ctx(t), phone, PurposeSignUp, code); !errors.Is(err, ErrCodeExpired) {
		t.Fatalf("Check(used code) = %v, want ErrCodeExpired", err)
	}
}

func TestCheckWithoutACodeSent(t *testing.T) {
	s, _, _, phone := newTest(t, 2)
	if err := s.Check(ctx(t), phone, PurposeSignUp, "123456"); !errors.Is(err, ErrCodeExpired) {
		t.Fatalf("Check(nothing sent) = %v, want ErrCodeExpired", err)
	}
}

func TestFiveWrongCodesEndTheCode(t *testing.T) {
	s, sender, _, phone := newTest(t, 3)
	if err := s.Send(ctx(t), phone, PurposeSignUp); err != nil {
		t.Fatal(err)
	}
	code := sender.last(t)
	for i := 1; i < MaxAttempts; i++ {
		if err := s.Check(ctx(t), phone, PurposeSignUp, wrong(code)); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("wrong code %d = %v, want ErrInvalidCode", i, err)
		}
	}
	if err := s.Check(ctx(t), phone, PurposeSignUp, wrong(code)); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("wrong code %d = %v, want ErrTooManyAttempts", MaxAttempts, err)
	}
	if err := s.Check(ctx(t), phone, PurposeSignUp, code); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("right code after too many tries = %v, want ErrTooManyAttempts", err)
	}
}

func TestCodeExpiresAfterFiveMinutes(t *testing.T) {
	s, sender, c, phone := newTest(t, 4)
	if err := s.Send(ctx(t), phone, PurposeSignUp); err != nil {
		t.Fatal(err)
	}
	c.advance(CodeTTL)
	if err := s.Check(ctx(t), phone, PurposeSignUp, sender.last(t)); !errors.Is(err, ErrCodeExpired) {
		t.Fatalf("Check(after %v) = %v, want ErrCodeExpired", CodeTTL, err)
	}
}

func TestOnlyTheNewestCodeWorks(t *testing.T) {
	s, sender, c, phone := newTest(t, 5)
	if err := s.Send(ctx(t), phone, PurposeSignUp); err != nil {
		t.Fatal(err)
	}
	first := sender.last(t)
	c.advance(ResendAfter)
	if err := s.Send(ctx(t), phone, PurposeSignUp); err != nil {
		t.Fatal(err)
	}
	second := sender.last(t)
	if first != second {
		if err := s.Check(ctx(t), phone, PurposeSignUp, first); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("Check(older code) = %v, want ErrInvalidCode", err)
		}
	}
	if err := s.Check(ctx(t), phone, PurposeSignUp, second); err != nil {
		t.Fatalf("Check(newest code) = %v, want nil", err)
	}
}

func TestANewCodeWaitsSixtySeconds(t *testing.T) {
	s, sender, c, phone := newTest(t, 6)
	if err := s.Send(ctx(t), phone, PurposeSignUp); err != nil {
		t.Fatal(err)
	}
	c.advance(ResendAfter - time.Second)
	if err := s.Send(ctx(t), phone, PurposeSignUp); !errors.Is(err, ErrResendTooSoon) {
		t.Fatalf("Send(after 59s) = %v, want ErrResendTooSoon", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("sent %d codes, want 1", len(sender.sent))
	}
	c.advance(time.Second)
	if err := s.Send(ctx(t), phone, PurposeSignUp); err != nil {
		t.Fatalf("Send(after 60s) = %v, want nil", err)
	}
}

func TestAtMostFiveCodesAnHour(t *testing.T) {
	s, _, c, phone := newTest(t, 7)
	for i := 0; i < MaxCodesPerHour; i++ {
		if err := s.Send(ctx(t), phone, PurposeSignUp); err != nil {
			t.Fatalf("code %d: %v", i+1, err)
		}
		c.advance(time.Minute)
	}
	if err := s.Send(ctx(t), phone, PurposeSignUp); !errors.Is(err, ErrTooManyCodes) {
		t.Fatalf("code %d within the hour = %v, want ErrTooManyCodes", MaxCodesPerHour+1, err)
	}
	// An hour after the first, it no longer counts.
	c.advance(time.Hour - time.Duration(MaxCodesPerHour)*time.Minute + time.Second)
	if err := s.Send(ctx(t), phone, PurposeSignUp); err != nil {
		t.Fatalf("code an hour after the first = %v, want nil", err)
	}
}

func TestACodeThatCouldNotBeSentDoesNotCount(t *testing.T) {
	s, sender, _, phone := newTest(t, 8)
	sender.err = errors.New("provider down")
	if err := s.Send(ctx(t), phone, PurposeSignUp); !errors.Is(err, ErrNotSent) {
		t.Fatalf("Send(provider down) = %v, want ErrNotSent", err)
	}
	if err := s.Check(ctx(t), phone, PurposeSignUp, "000000"); !errors.Is(err, ErrCodeExpired) {
		t.Fatalf("Check(unsent code) = %v, want ErrCodeExpired", err)
	}
	sender.err = nil
	if err := s.Send(ctx(t), phone, PurposeSignUp); err != nil {
		t.Fatalf("Send(right after a failed send) = %v, want nil", err)
	}
}

func TestACodeWorksOnceEvenWhenCheckedAtOnce(t *testing.T) {
	s, sender, _, phone := newTest(t, 9)
	if err := s.Send(ctx(t), phone, PurposeSignUp); err != nil {
		t.Fatal(err)
	}
	code := sender.last(t)
	// Open the pool's connections first (checks on a number with no code),
	// so the real checks below run at the same time instead of queueing
	// behind new connections.
	var warm sync.WaitGroup
	for i := 0; i < 10; i++ {
		warm.Add(1)
		go func() { defer warm.Done(); _ = s.Check(context.Background(), phone+"0", PurposeSignUp, code) }()
	}
	warm.Wait()
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- s.Check(context.Background(), phone, PurposeSignUp, code)
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	ok := 0
	for err := range results {
		if err == nil {
			ok++
		} else if !errors.Is(err, ErrCodeExpired) {
			t.Errorf("Check = %v, want nil or ErrCodeExpired", err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d checks succeeded, want exactly 1", ok)
	}
}
