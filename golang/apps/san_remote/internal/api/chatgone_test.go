package api

import (
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/chatgpt"
	"github.com/wargasipil/san_remote/internal/claude"
	"github.com/wargasipil/san_remote/internal/gemini"
)

// A chat link that opens no chat (deleted on the site) is NotFound, with
// "the chat is not there" in its message, which the tower reads.
func TestChatGoneIsNotFound(t *testing.T) {
	a := &API{}
	page := errors.New("ChatGPT did not respond as expected: waited 30s for the chat's turns")
	for _, gone := range []error{chatgpt.ErrChatGone, gemini.ErrChatGone, claude.ErrChatGone} {
		err := a.fail(fmt.Errorf("%w: https://chatgpt.com/c/x, deleted or not this account's (%w)", gone, page))
		var ce *connect.Error
		if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
			t.Errorf("%v: %v", gone, err)
		}
	}
}
