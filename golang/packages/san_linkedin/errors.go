package san_linkedin

import "errors"

var (
	// ErrNoTitle is returned for an article without a `# ` heading.
	ErrNoTitle = errors.New("san_linkedin: the article has no `# ` title")

	// ErrNotLoggedIn means the browser profile has no LinkedIn session. Run
	// the login once, by hand, against the same profile.
	ErrNotLoggedIn = errors.New("san_linkedin: not logged in to LinkedIn")
)
