package go_gpt

import "errors"

var (
	// ErrNeedsHuman reports a browser state that retrying cannot clear: the
	// session expired, a Cloudflare interstitial is up, or a verification
	// checkbox is waiting. Someone has to look at the browser. It is the one
	// failure a caller must treat differently from every other.
	ErrNeedsHuman = errors.New("go_gpt: browser needs a human")

	// ErrClosed is returned once the conversation, its client, or the browser
	// behind them is gone.
	ErrClosed = errors.New("go_gpt: closed")

	// ErrGenerating is returned by Send when the composer is still disabled
	// because a response is streaming. The turn-taking loop cannot provoke
	// this; a concurrent caller can.
	ErrGenerating = errors.New("go_gpt: a response is still generating")

	// ErrStreamStalled reports a turn that produced no further data before the
	// idle timeout, or never completed before the turn timeout.
	ErrStreamStalled = errors.New("go_gpt: response stream stalled")

	// ErrEmptyPrompt is returned by Send for a prompt that is empty or only
	// whitespace, which the composer would silently refuse to submit.
	ErrEmptyPrompt = errors.New("go_gpt: empty prompt")
)
