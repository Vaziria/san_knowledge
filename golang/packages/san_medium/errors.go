package san_medium

import "errors"

var (
	// ErrNoTitle is returned for an article without a `# ` heading.
	ErrNoTitle = errors.New("san_medium: the article has no `# ` title")

	// ErrNotLoggedIn means the browser profile has no Medium session. Run the
	// login once, by hand, against the same profile.
	ErrNotLoggedIn = errors.New("san_medium: not logged in to Medium")

	// ErrBlocked means Cloudflare, in front of Medium, shows "Sorry, you have
	// been blocked" to this browser. It is a firewall decision, not a check a
	// person can pass. It was seen whenever chromedp itself launched Chrome,
	// and then it stuck to the Medium sign-in session, while a fresh profile
	// on the same connection was let in. Sign in again on a new profile.
	ErrBlocked = errors.New("san_medium: Cloudflare blocks this browser from Medium")

	// ErrNeedsHuman reports a browser state retrying cannot clear, such as a
	// verification challenge or an editor that never appeared. Someone has to
	// look at the browser window.
	ErrNeedsHuman = errors.New("san_medium: the browser needs a human")
)
