package san_tiktok

import "errors"

var (
	// ErrNotLoggedIn means the browser profile has no TikTok session. Run the
	// login once, by hand, against the same profile.
	ErrNotLoggedIn = errors.New("san_tiktok: not logged in to TikTok")

	// ErrStudio means TikTok Studio did not do what a step expected: an
	// element never appeared, or a change did not take. The page may have been
	// redesigned, or something in the window got in the way.
	ErrStudio = errors.New("san_tiktok: TikTok Studio did not respond as expected")

	// ErrNeedsHuman reports a page state the tool cannot clear itself, such as
	// a captcha or a verification check. Someone has to look at the browser
	// window.
	ErrNeedsHuman = errors.New("san_tiktok: the browser needs a human")
)
