package medium

import (
	"errors"
	"regexp"
	"testing"
	"time"
)

func TestStoryID(t *testing.T) {
	for _, s := range []string{"a379bedbcab6", "https://medium.com/p/a379bedbcab6/edit", "https://medium.com/p/a379bedbcab6/edit?source=x",
		"https://medium.com/@getresolvedid/uji-san-remote-a379bedbcab6"} {
		if id, err := StoryID(s); err != nil || id != "a379bedbcab6" {
			t.Errorf("StoryID(%q) = %q, %v", s, id, err)
		}
	}
	if _, err := StoryID("not a story"); !errors.Is(err, ErrInvalid) {
		t.Errorf("not a story: %v", err)
	}
}

func TestCheckSchedule(t *testing.T) {
	now := time.Date(2026, 10, 25, 4, 17, 0, 0, time.Local)
	at := time.Date(2026, 11, 24, 9, 0, 0, 0, time.Local)
	for name, c := range map[string]struct {
		at     time.Time
		topics []string
		want   error
	}{
		"fine":        {at, []string{"Programming"}, nil},
		"too soon":    {now.Add(5 * time.Minute).Truncate(time.Minute), nil, ErrTooSoon},
		"seconds":     {at.Add(30 * time.Second), nil, ErrInvalid},
		"six topics":  {at, []string{"a", "b", "c", "d", "e", "f"}, ErrInvalid},
		"empty topic": {at, []string{" "}, ErrInvalid},
	} {
		if err := checkSchedule(c.at, c.topics, now); !errors.Is(err, c.want) || (c.want == nil && err != nil) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
}

// Medium's ways of showing a time, as seen on 2026-10-25.
func TestTimes(t *testing.T) {
	got, err := parseShown("11/24/2026,\n\n9:00 AM GMT+7")
	if err != nil || !got.Equal(time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC)) {
		t.Errorf("parseShown = %v, %v", got, err)
	}
	if _, err := parseShown("tomorrow"); !errors.Is(err, ErrPage) {
		t.Errorf("parseShown of nonsense: %v", err)
	}
	now := time.Date(2026, 10, 25, 4, 17, 0, 0, time.Local)
	if at, err := scheduledAt("Nov 24, 9:00 AM", now); err != nil || !at.Equal(time.Date(2026, 11, 24, 9, 0, 0, 0, time.Local)) {
		t.Errorf("scheduledAt = %v, %v", at, err)
	}
	if at, _ := scheduledAt("Jan 5, 9:00 AM", now); at.Year() != 2027 {
		t.Errorf("January seen in October is next year's: %v", at)
	}
	if n := wordCount("1 min read (1,031 words)"); n != 1031 {
		t.Errorf("wordCount = %d", n)
	}
}

// The helpers service.go calls are the ones medium.js has.
func TestHelpersExist(t *testing.T) {
	for _, fn := range []string{"state", "account", "pressWrite", "pressStories", "typeTitle", "titleText", "caretInBody", "paste",
		"find", "emptyGraf", "empty", "plusAt", "pressPlus", "menuReady", "stage", "chooseImage", "figure", "pressImage", "pressAlt",
		"altEditorFocused", "altText", "pressAltSave", "pasteText", "saveState", "snapshot", "pressButton", "hasButton", "topics",
		"pressRemoveTopic", "typeTopic", "suggestions", "pressSuggestion", "scheduleShown", "pressDate", "pickerOpen", "pickerMonth",
		"pressNextMonth", "pressPick", "closePicker", "stories", "storiesReady", "scrollEnd", "hasRow", "pressRowMenu", "menuItems",
		"pressMenuItem", "pressDialogButton"} {
		if !regexp.MustCompile(`(?m)^    (async )?` + fn + `[(:]`).MatchString(mediumJS) {
			t.Errorf("medium.js has no %s", fn)
		}
	}
}
