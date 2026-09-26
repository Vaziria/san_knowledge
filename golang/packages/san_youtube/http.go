package san_youtube

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	// maxBody bounds one response. A watch page is about 1.5 MB.
	maxBody = 16 << 20

	backoffBase = time.Second
	backoffMax  = 30 * time.Second
)

// response is a finished HTTP exchange. url is where redirects ended, which
// is how the consent page and google.com/sorry are recognised.
type response struct {
	status int
	body   []byte
	url    *url.URL
	header http.Header
}

// send runs one request, retrying network errors, 5xx and 429 with
// exponential backoff. Any other status is returned for the caller to judge,
// because a 404 means different things on different pages.
func (c *Client) send(ctx context.Context, method, rawURL string, body []byte, hdr http.Header) (*response, error) {
	var last error
	for attempt := 0; ; attempt++ {
		resp, err := c.once(ctx, method, rawURL, body, hdr)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		switch {
		case err != nil:
			last = fmt.Errorf("san_youtube: %s %s: %w", method, stripQuery(rawURL), err)
		case resp.status == http.StatusTooManyRequests:
			c.limited.Store(c.opts.now().UnixNano())
			last = fmt.Errorf("%w: HTTP 429 from %s", ErrRateLimited, stripQuery(resp.url.String()))
		case resp.status >= 500:
			last = &StatusError{Code: resp.status, URL: stripQuery(rawURL)}
		default:
			return resp, nil
		}
		if attempt >= c.opts.retries {
			return nil, last
		}
		if err := c.opts.sleep(ctx, backoff(attempt)); err != nil {
			return nil, err
		}
	}
}

func (c *Client) once(ctx context.Context, method, rawURL string, body []byte, hdr http.Header) (*response, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.opts.userAgent)
	req.Header.Set("Accept-Language", c.acceptLanguage())
	if ck := c.cookieHeader(); ck != "" {
		req.Header.Set("Cookie", ck)
	}
	for k, v := range hdr {
		req.Header[k] = v
	}

	resp, err := c.opts.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	// Where redirects ended. http.Transport records it; a RoundTripper of the
	// caller's own may not.
	final := req.URL
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL
	}
	return &response{status: resp.StatusCode, body: data, url: final, header: resp.Header}, nil
}

// backoff is 1s, 2s, 4s … capped at 30s.
func backoff(attempt int) time.Duration {
	d := backoffBase << min(attempt, 10)
	return min(d, backoffMax)
}

// get fetches a page, getting past the EU cookie consent page once if YouTube
// shows it despite the SOCS cookie sent with every request.
func (c *Client) get(ctx context.Context, rawURL string) (*response, error) {
	resp, err := c.send(ctx, http.MethodGet, rawURL, nil, nil)
	if err != nil || !isConsent(resp) {
		return resp, err
	}
	if err := c.consent(ctx, resp); err != nil {
		return nil, err
	}
	resp, err = c.send(ctx, http.MethodGet, rawURL, nil, nil)
	if err != nil {
		return nil, err
	}
	if isConsent(resp) {
		return nil, ErrConsent
	}
	return resp, nil
}

var (
	reForm   = regexp.MustCompile(`(?is)<form\b([^>]*)>(.*?)</form>`)
	reInput  = regexp.MustCompile(`(?is)<input\b[^>]*>`)
	reAttr   = regexp.MustCompile(`(?is)\b(action|name|value|type)\s*=\s*"([^"]*)"`)
	reReject = regexp.MustCompile(`(?i)reject`)
)

type consentForm struct {
	action string
	values url.Values
	reject bool
}

// consentForms finds the forms on a consent page: the ones that post to a
// consent host's /save. The page has one per button, "Reject all" and
// "Accept all", each carrying its choice in hidden inputs.
func consentForms(page []byte) []consentForm {
	var forms []consentForm
	for _, m := range reForm.FindAllSubmatch(page, -1) {
		attrs := attrMap(m[1])
		action := attrs["action"]
		u, err := url.Parse(action)
		if err != nil || !strings.Contains(u.Host+u.Path, "consent") || !strings.HasSuffix(u.Path, "/save") {
			continue
		}
		f := consentForm{action: action, values: url.Values{}, reject: reReject.Match(m[2])}
		for _, in := range reInput.FindAll(m[2], -1) {
			a := attrMap(in)
			if a["name"] != "" && (a["type"] == "" || strings.EqualFold(a["type"], "hidden")) {
				f.values.Add(a["name"], a["value"])
			}
		}
		forms = append(forms, f)
	}
	return forms
}

func attrMap(tag []byte) map[string]string {
	attrs := map[string]string{}
	for _, a := range reAttr.FindAllSubmatch(tag, -1) {
		attrs[strings.ToLower(string(a[1]))] = html.UnescapeString(string(a[2]))
	}
	return attrs
}

func isConsent(resp *response) bool {
	return strings.HasPrefix(resp.url.Host, "consent.") || len(consentForms(resp.body)) > 0
}

// consent submits the consent page's form, preferring "Reject all", and keeps
// the cookies the answer sets. The redirect that follows is not taken: it only
// leads back to the page get is about to fetch again.
func (c *Client) consent(ctx context.Context, page *response) error {
	forms := consentForms(page.body)
	if len(forms) == 0 {
		return ErrConsent
	}
	form := forms[0]
	for _, f := range forms {
		if f.reject {
			form = f
			break
		}
	}
	action, err := page.url.Parse(form.action)
	if err != nil {
		return fmt.Errorf("%w: bad form action %q", ErrConsent, form.action)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, action.String(), strings.NewReader(form.values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.opts.userAgent)
	req.Header.Set("Accept-Language", c.acceptLanguage())

	noRedirect := *c.opts.client
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := noRedirect.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %v", ErrConsent, err)
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, maxBody))
	resp.Body.Close()

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		return fmt.Errorf("%w: the form set no cookie (HTTP %d)", ErrConsent, resp.StatusCode)
	}
	c.mu.Lock()
	for _, ck := range cookies {
		c.cookies[ck.Name] = ck.Value
	}
	c.mu.Unlock()
	return nil
}

func (c *Client) cookieHeader() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	parts := make([]string, 0, len(c.cookies))
	for k, v := range c.cookies {
		parts = append(parts, k+"="+v)
	}
	// Sorted, so requests are the same from one run to the next.
	slices.Sort(parts)
	return strings.Join(parts, "; ")
}

func (c *Client) acceptLanguage() string {
	lang := c.opts.lang
	if lang == "en" || strings.HasPrefix(lang, "en-") {
		return "en-US,en;q=0.9"
	}
	return lang + ",en;q=0.8"
}

func stripQuery(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.RawQuery, u.Fragment = "", ""
	return u.String()
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
