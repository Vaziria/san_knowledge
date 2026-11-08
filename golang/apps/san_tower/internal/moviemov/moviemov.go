// Package moviemov is a client of the API of a site made from web_movie
// (php/apps/web_movie: MovieMov, https://moviemov.id; its docs/API.md and
// docs/openapi.json), over plain net/http, logged in as an admin: what the
// tower's MovieMov connection and steps use.
//
// The site gives a Sanctum token for an email and password (POST /login, 5
// tries a minute from one address), which never expires. Sessions keeps one
// per connection, in memory, so a step does not log in on every run: a
// connection whose base URI, username or password changed logs in anew,
// and a token the site refuses (401, or 403 on a public route) is replaced
// once.
package moviemov

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultBase is MovieMov's own API.
const DefaultBase = "https://moviemov.id/api"

// DeviceName names the tokens the tower gets, as the site lists them.
const DeviceName = "san-tower"

// LoginsPerMinute is the site's limit on POST /login from one address
// (throttle:5,1): the tower keeps under it on its own.
const LoginsPerMinute = 5

// MaxThumbnail is the largest thumbnail file the site takes: 2 MB, as
// Laravel counts it (max:2048, in kilobytes).
const MaxThumbnail = 2048 * 1024

// Login is an admin's login on a site.
type Login struct {
	Base, Email, Password string
}

// key tells logins apart without keeping the password in clear: a token is
// kept for one connection and one key.
func (l Login) key() string {
	sum := sha256.Sum256([]byte(l.Base + "\x00" + l.Email + "\x00" + l.Password))
	return hex.EncodeToString(sum[:])
}

// User is the admin a token is for.
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Error is the site refusing a call: its status, its message, and for a
// 422 the fields it named, in its order.
type Error struct {
	Status  int
	Message string
	Fields  []FieldError
}

// FieldError is a field the site refused, with its messages.
type FieldError struct {
	Field    string
	Messages []string
}

// Error says each field and its messages when the site named fields
// ("MovieMov: embed_url: The embed url field must be a valid URL. (422)"),
// else the message.
func (e *Error) Error() string {
	if len(e.Fields) == 0 {
		return fmt.Sprintf("MovieMov: %s (%d)", e.Message, e.Status)
	}
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + strings.Join(f.Messages, " ")
	}
	return fmt.Sprintf("MovieMov: %s (%d)", strings.Join(parts, "; "), e.Status)
}

// IsStatus says whether err is the site answering status.
func IsStatus(err error, status int) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == status
}

// ErrTooManyLogins is the tower holding a login back: it logged in to the
// site LoginsPerMinute times in the last minute.
var ErrTooManyLogins = errors.New("too many logins")

// Sessions keeps the tokens of the tower's MovieMov connections, by
// connection id, each with the key of the login it was got with.
type Sessions struct {
	// HTTP is the client calls go through; a 60 s timeout when nil.
	HTTP *http.Client
	// Now is the clock, for the login limit; time.Now when nil.
	Now func() time.Time

	mu      sync.Mutex
	tokens  map[uint32]token
	logins  map[string][]time.Time // when the tower logged in, by base URI
	loginMu sync.Mutex             // one login at a time
}

type token struct {
	key, value string
	user       User
}

func NewSessions() *Sessions { return &Sessions{} }

func (s *Sessions) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Sessions) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return defaultHTTP
}

var defaultHTTP = &http.Client{Timeout: 60 * time.Second}

// Client is the API of connection id's site, logged in with l. Log, when
// not nil, gets a line for each login, never the password or the token.
func (s *Sessions) Client(id uint32, l Login, log func(string)) *Client {
	l.Base = strings.TrimRight(strings.TrimSpace(l.Base), "/")
	if l.Base == "" {
		l.Base = DefaultBase
	}
	return &Client{s: s, id: id, login: l, log: log}
}

// Forget drops connection id's token, as when the connection is deleted.
func (s *Sessions) Forget(id uint32) {
	s.mu.Lock()
	delete(s.tokens, id)
	s.mu.Unlock()
}

// Client calls one site as one admin.
type Client struct {
	s     *Sessions
	id    uint32
	login Login
	log   func(string)
}

func (c *Client) logf(format string, args ...any) {
	if c.log != nil {
		c.log(fmt.Sprintf(format, args...))
	}
}

// Base is the site's API the client calls.
func (c *Client) Base() string { return c.login.Base }

// token is the connection's token for the client's login: the kept one,
// unless it is stale (the one the site just refused), else a new login;
// fresh says it is a new one.
func (c *Client) token(ctx context.Context, stale string) (t token, fresh bool, err error) {
	key := c.login.key()
	kept := func() (token, bool) {
		c.s.mu.Lock()
		defer c.s.mu.Unlock()
		t, ok := c.s.tokens[c.id]
		return t, ok && t.key == key && t.value != stale
	}
	if t, ok := kept(); ok {
		return t, false, nil
	}
	c.s.loginMu.Lock()
	defer c.s.loginMu.Unlock()
	if t, ok := kept(); ok { // another run logged in meanwhile
		return t, false, nil
	}
	if t, err = c.logIn(ctx); err != nil {
		return token{}, false, err
	}
	c.s.mu.Lock()
	if c.s.tokens == nil {
		c.s.tokens = make(map[uint32]token)
	}
	c.s.tokens[c.id] = t
	c.s.mu.Unlock()
	return t, true, nil
}

// logIn asks the site for a token (POST /login), unless the tower already
// logged in to it LoginsPerMinute times in the last minute.
func (c *Client) logIn(ctx context.Context) (token, error) {
	if c.login.Email == "" || c.login.Password == "" {
		return token{}, errors.New("MovieMov: the connection needs the admin's email and password")
	}
	now := c.s.now()
	c.s.mu.Lock()
	recent := slices.DeleteFunc(slices.Clone(c.s.logins[c.login.Base]), func(at time.Time) bool { return now.Sub(at) >= time.Minute })
	if len(recent) >= LoginsPerMinute {
		c.s.mu.Unlock()
		wait := (time.Minute - now.Sub(recent[0]) + time.Second - 1) / time.Second
		return token{}, fmt.Errorf("MovieMov: %w: the tower tried to log in to %s %d times in the last minute, the most the site allows; try again in %ds", ErrTooManyLogins, c.login.Base, LoginsPerMinute, wait)
	}
	if c.s.logins == nil {
		c.s.logins = make(map[string][]time.Time)
	}
	c.s.logins[c.login.Base] = append(recent, now)
	c.s.mu.Unlock()

	body, _ := json.Marshal(map[string]string{"email": c.login.Email, "password": c.login.Password, "device_name": DeviceName})
	var res struct {
		Token string `json:"token"`
		User  User   `json:"user"`
	}
	_, err := c.send(ctx, http.MethodPost, "/login", nil, "application/json", body, "", &res)
	var e *Error
	switch {
	case errors.As(err, &e) && e.Status == http.StatusUnprocessableEntity:
		return token{}, fmt.Errorf("MovieMov login as %s at %s: %w: check the connection's username and password (only admins can log in)", c.login.Email, c.login.Base, err)
	case errors.As(err, &e) && e.Status == http.StatusTooManyRequests:
		return token{}, fmt.Errorf("MovieMov login as %s at %s: %w: the site takes 5 logins a minute from one address; wait a minute", c.login.Email, c.login.Base, err)
	case errors.As(err, &e) && (e.Status == http.StatusNotFound || e.Status == http.StatusMethodNotAllowed):
		return token{}, fmt.Errorf("MovieMov login at %s: %w: is the Base URI the site's API, such as %s?", c.login.Base, err, DefaultBase)
	case err != nil:
		return token{}, fmt.Errorf("MovieMov login at %s: %w", c.login.Base, err)
	case res.Token == "":
		return token{}, fmt.Errorf("MovieMov login at %s: the site gave no token: is the Base URI the site's API, such as %s?", c.login.Base, DefaultBase)
	}
	c.logf("logged in to %s as %s (a new token, %s)", c.login.Base, res.User.Email, DeviceName)
	return token{key: c.login.key(), value: res.Token, user: res.User}, nil
}

// request is a call's body, made once so it can be sent again.
type request struct {
	contentType string
	body        []byte
}

// jsonBody is v as a JSON body.
func jsonBody(v any) (request, error) {
	b, err := json.Marshal(v)
	return request{contentType: "application/json", body: b}, err
}

// call sends method path with query and the body, with the token, and
// decodes the answer into result when it is not nil. A kept token the site
// refuses is replaced by a new login, once; a new one it refuses is the
// error. Refusing is 401, or 403 on a public route such as GET /movies,
// which reads a revoked token as no token: "Only admins can list drafts and
// scheduled movies." (only admins can log in, so a new token is an
// admin's).
func (c *Client) call(ctx context.Context, method, p string, query url.Values, r request, result any) (int, error) {
	t, fresh, err := c.token(ctx, "")
	if err != nil {
		return 0, err
	}
	status, err := c.send(ctx, method, p, query, r.contentType, r.body, t.value, result)
	if (status != http.StatusUnauthorized && status != http.StatusForbidden) || fresh {
		return status, err
	}
	c.logf("the site refused the kept token (%d): logging in again", status)
	if t, _, err = c.token(ctx, t.value); err != nil {
		return 0, err
	}
	return c.send(ctx, method, p, query, r.contentType, r.body, t.value, result)
}

// send sends one request. The token goes in a header, never in the URL, so
// it stays out of errors.
func (c *Client) send(ctx context.Context, method, p string, query url.Values, contentType string, body []byte, tok string, result any) (int, error) {
	u := c.login.Base + p
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return 0, fmt.Errorf("MovieMov: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "san_tower")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := c.s.client().Do(req)
	if err != nil {
		return 0, fmt.Errorf("MovieMov does not answer at %s: %w", c.login.Base, unwrapURL(err))
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return res.StatusCode, fmt.Errorf("MovieMov: reading the answer: %w", err)
	}
	if res.StatusCode >= 300 {
		return res.StatusCode, siteError(res, data)
	}
	if result != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, result); err != nil {
			return res.StatusCode, fmt.Errorf("MovieMov answered %s with something else than JSON (%s): is the Base URI the site's API, such as %s?", req.URL.Path, snippet(data), DefaultBase)
		}
	}
	return res.StatusCode, nil
}

// unwrapURL drops url.Error's "Post \"https://...\":", which repeats the
// address.
func unwrapURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// siteError reads an error answer: Laravel's {"message": ..., "errors":
// {"field": ["..."]}}, the fields in the order the site gave them.
func siteError(res *http.Response, data []byte) *Error {
	e := &Error{Status: res.StatusCode}
	var body struct {
		Message string          `json:"message"`
		Errors  json.RawMessage `json:"errors"`
	}
	if json.Unmarshal(data, &body) != nil {
		e.Message = res.Status + ", not JSON (" + snippet(data) + ")"
		return e
	}
	e.Message = body.Message
	if e.Message == "" {
		e.Message = res.Status
	}
	e.Fields = fieldErrors(body.Errors)
	return e
}

// fieldErrors reads a 422's errors object in its order.
func fieldErrors(raw json.RawMessage) []FieldError {
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil
	}
	var out []FieldError
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return out
		}
		name, _ := t.(string)
		var msgs []string
		var one string
		var v json.RawMessage
		if dec.Decode(&v) != nil {
			return out
		}
		if json.Unmarshal(v, &msgs) != nil && json.Unmarshal(v, &one) == nil {
			msgs = []string{one}
		}
		out = append(out, FieldError{Field: name, Messages: msgs})
	}
	return out
}

// snippet is the start of an answer that is not JSON, on one line.
func snippet(data []byte) string {
	s := strings.Join(strings.Fields(string(data)), " ")
	if len(s) > 80 {
		s = s[:80] + "..."
	}
	if s == "" {
		return "empty"
	}
	return s
}

// Me is the admin the token is for (GET /me), logging in first when the
// connection has no token yet.
func (c *Client) Me(ctx context.Context) (User, error) {
	var u User
	_, err := c.call(ctx, http.MethodGet, "/me", nil, request{}, &u)
	return u, err
}

// File is a file to upload.
type File struct {
	Name string
	Data []byte
}

// Movie is what a create or an update sends: Fields by the API's names
// (title, embed_url, ...), each a string, an int, a bool, a []string or nil
// (JSON null), and Thumbnail, a file, which makes it a multipart request.
type Movie struct {
	Fields    map[string]any
	Thumbnail *File
}

// body is m as JSON, or as multipart with its thumbnail: a list's items as
// name[] (PHP reads them as an array, so a comma in a category stays), an
// empty list as an empty value (which the site reads as none), a bool as 1
// or 0.
func (m Movie) body() (request, error) {
	if m.Thumbnail == nil {
		return jsonBody(m.Fields)
	}
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	names := make([]string, 0, len(m.Fields))
	for name := range m.Fields {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		var err error
		switch v := m.Fields[name].(type) {
		case nil:
			err = w.WriteField(name, "")
		case string:
			err = w.WriteField(name, v)
		case int:
			err = w.WriteField(name, strconv.Itoa(v))
		case bool:
			err = w.WriteField(name, map[bool]string{true: "1", false: "0"}[v])
		case []string:
			if len(v) == 0 {
				err = w.WriteField(name, "")
			}
			for _, item := range v {
				if err == nil {
					err = w.WriteField(name+"[]", item)
				}
			}
		default:
			err = fmt.Errorf("field %s: %T cannot go in a form", name, v)
		}
		if err != nil {
			return request{}, err
		}
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="thumbnail"; filename=%q`, path.Base(m.Thumbnail.Name)))
	h.Set("Content-Type", ImageType(m.Thumbnail.Data))
	part, err := w.CreatePart(h)
	if err != nil {
		return request{}, err
	}
	if _, err := part.Write(m.Thumbnail.Data); err != nil {
		return request{}, err
	}
	if err := w.Close(); err != nil {
		return request{}, err
	}
	return request{contentType: w.FormDataContentType(), body: b.Bytes()}, nil
}

// ImageType is the picture's MIME type by its first bytes: image/jpeg,
// image/png or image/webp, the ones the site takes, else "".
func ImageType(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	}
	return ""
}

// movieAnswer is the site's {"data": Movie}.
type movieAnswer struct {
	Data json.RawMessage `json:"data"`
}

// CreateMovie creates a movie (POST /movies) and returns it as the site
// gives it.
func (c *Client) CreateMovie(ctx context.Context, m Movie) (json.RawMessage, error) {
	r, err := m.body()
	if err != nil {
		return nil, err
	}
	var a movieAnswer
	_, err = c.call(ctx, http.MethodPost, "/movies", nil, r, &a)
	return a.Data, err
}

// UpdateMovie changes the fields m has of movie slug: PATCH with JSON, or
// POST with multipart when it has a thumbnail file (PHP reads uploaded
// files on POST only).
func (c *Client) UpdateMovie(ctx context.Context, slug string, m Movie) (json.RawMessage, error) {
	r, err := m.body()
	if err != nil {
		return nil, err
	}
	method := http.MethodPatch
	if m.Thumbnail != nil {
		method = http.MethodPost
	}
	var a movieAnswer
	_, err = c.call(ctx, method, "/movies/"+url.PathEscape(slug), nil, r, &a)
	return a.Data, err
}

// Page is a page of a movie list.
type Page struct {
	Data []json.RawMessage `json:"data"`
	Meta struct {
		CurrentPage int `json:"current_page"`
		LastPage    int `json:"last_page"`
		PerPage     int `json:"per_page"`
		Total       int `json:"total"`
	} `json:"meta"`
}

// Movies lists movies (GET /movies) with query: q, category, tags, sort,
// dir, status, per_page, page.
func (c *Client) Movies(ctx context.Context, query url.Values) (Page, error) {
	var p Page
	_, err := c.call(ctx, http.MethodGet, "/movies", query, request{}, &p)
	return p, err
}

// DeleteMovie deletes movie slug (DELETE /movies/{slug}); one not there is
// an *Error of status 404.
func (c *Client) DeleteMovie(ctx context.Context, slug string) error {
	_, err := c.call(ctx, http.MethodDelete, "/movies/"+url.PathEscape(slug), nil, request{}, nil)
	return err
}

// Reports lists the movies with open broken-video reports, most reported
// first (GET /reports).
func (c *Client) Reports(ctx context.Context) ([]json.RawMessage, error) {
	var a struct {
		Data []json.RawMessage `json:"data"`
	}
	_, err := c.call(ctx, http.MethodGet, "/reports", nil, request{}, &a)
	return a.Data, err
}

// Categories lists the site's categories by name (GET /categories, a
// public list: no login): id, name, slug, description, movies_count (of
// published movies) and url.
func (c *Client) Categories(ctx context.Context) ([]json.RawMessage, error) {
	return c.public(ctx, "/categories")
}

// Tags lists the tags of the site's published movies, the most used first
// (GET /tags, a public list: no login): id, name, slug, movies_count and
// url.
func (c *Client) Tags(ctx context.Context) ([]json.RawMessage, error) {
	return c.public(ctx, "/tags")
}

// public reads a public list's data, without a token.
func (c *Client) public(ctx context.Context, p string) ([]json.RawMessage, error) {
	var a struct {
		Data []json.RawMessage `json:"data"`
	}
	_, err := c.send(ctx, http.MethodGet, p, nil, "", nil, "", &a)
	return a.Data, err
}

// ResolveReport marks every open report of movie slug fixed (POST
// /reports/{slug}/resolve) and says how many there were.
func (c *Client) ResolveReport(ctx context.Context, slug string) (int, error) {
	var a struct {
		Resolved int `json:"resolved"`
	}
	_, err := c.call(ctx, http.MethodPost, "/reports/"+url.PathEscape(slug)+"/resolve", nil, request{}, &a)
	return a.Resolved, err
}

// slugText is a slug as the site makes them (alpha_dash).
var slugText = regexp.MustCompile(`^[\pL\pM\pN_-]+$`)

// Slug reads a movie as people give it: its slug (night-train), or its
// page's link (https://moviemov.id/movies/night-train, /en/movies/...,
// with a query or #), and returns the slug.
func Slug(s string) (string, error) {
	raw := strings.TrimSpace(s)
	slug := raw
	if i := strings.Index(slug, "/movies/"); i >= 0 {
		slug = slug[i+len("/movies/"):]
	}
	if i := strings.IndexAny(slug, "?#"); i >= 0 {
		slug = slug[:i]
	}
	slug = strings.Trim(slug, "/")
	if u, err := url.PathUnescape(slug); err == nil {
		slug = u
	}
	if !slugText.MatchString(slug) {
		return "", fmt.Errorf("%q is not a movie: give its slug, such as night-train, or its page's link", raw)
	}
	return slug, nil
}
