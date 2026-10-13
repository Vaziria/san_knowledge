package studio

import "errors"

var (
	// ErrNotLoggedIn means the browser profile has no YouTube session. Run
	// the login once, by hand, against the same profile.
	ErrNotLoggedIn = errors.New("san_youtube: not signed in to YouTube")

	// ErrStudio means YouTube Studio did not do what a step expected: an
	// element never appeared, or a change did not take. Studio may have been
	// redesigned, or something in the window got in the way. A video Studio
	// had already made is kept by it, as private.
	ErrStudio = errors.New("san_youtube: YouTube Studio did not respond as expected")

	// ErrTooSoon is returned for a publish time less than MinLead from now.
	ErrTooSoon = errors.New("san_youtube: the publish time is too soon")

	// ErrInvalidPost is returned for a post YouTube would refuse: no title,
	// a title or description too long or with < or >, or too many tags.
	ErrInvalidPost = errors.New("san_youtube: the post is not valid")
)
