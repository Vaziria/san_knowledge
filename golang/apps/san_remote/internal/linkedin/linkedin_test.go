package linkedin

import (
	"errors"
	"regexp"
	"testing"
	"time"
)

func TestStoryID(t *testing.T) {
	for _, s := range []string{"7515804761766535168", "https://www.linkedin.com/article/edit/7515804761766535168/", "urn:li:fsd_firstPartyArticle:7515804761766535168"} {
		if id, err := StoryID(s); err != nil || id != "7515804761766535168" {
			t.Errorf("StoryID(%q) = %q, %v", s, id, err)
		}
	}
	if _, err := StoryID("not a draft"); !errors.Is(err, ErrInvalid) {
		t.Errorf("not a draft: %v", err)
	}
}

func TestCheckSchedule(t *testing.T) {
	now := time.Date(2026, 10, 25, 4, 17, 0, 0, time.Local)
	for at, want := range map[time.Time]error{
		time.Date(2026, 11, 20, 9, 0, 0, 0, time.Local):  nil,
		time.Date(2026, 11, 20, 9, 45, 0, 0, time.Local): nil,
		time.Date(2026, 11, 20, 9, 10, 0, 0, time.Local): ErrInvalid,
		time.Date(2026, 10, 25, 4, 30, 0, 0, time.Local): ErrTooSoon,
	} {
		if err := checkSchedule(at, now); !errors.Is(err, want) || (want == nil && err != nil) {
			t.Errorf("%s: %v, want %v", at.Format("Jan 2 15:04"), err, want)
		}
	}
}

// LinkedIn's ways of showing a time, as san_linkedin saw them on 2026-10-13.
func TestTimes(t *testing.T) {
	day, clock, zone, err := parseStated("Fri, Nov 20, 9:00 AM Western Indonesia Time, based on your location")
	if err != nil || day != "Fri, Nov 20" || clock != "9:00 AM" || zone != "Western Indonesia Time" {
		t.Errorf("parseStated = %q %q %q, %v", day, clock, zone, err)
	}
	at := time.Date(2026, 11, 20, 9, 0, 0, 0, time.Local)
	if posting(at) != "Fri, Nov 20 at 9:00 AM" || whenOf("Posting Fri, Nov 20 at 9:00 AM") != posting(at) {
		t.Errorf("posting = %q", posting(at))
	}
	now := time.Date(2026, 10, 25, 4, 17, 0, 0, time.Local)
	if got, err := scheduledAt("Fri, Nov 20 at 9:00 AM", now); err != nil || !got.Equal(at) {
		t.Errorf("scheduledAt = %v, %v", got, err)
	}
	// January 2027 starts on a Friday.
	if got, _ := scheduledAt("Fri, Jan 1 at 9:00 AM", now); got.Year() != 2027 {
		t.Errorf("a January seen in October: %v", got)
	}
}

// The helpers service.go calls are the ones linkedin.js has.
func TestHelpersExist(t *testing.T) {
	for _, fn := range []string{"state", "account", "pressWrite", "typeTitle", "titleText", "caretInBody", "paste", "find",
		"emptyBlock", "caretInEmpty", "caretIn", "backspace", "stage", "chooseFile", "cover", "media", "pressMediaButton",
		"pressMediaIcon", "typeAlt", "altText", "figure", "typeCaption", "saveState", "snapshot", "pressNext", "stillSaving",
		"share", "typeShare", "pressClock", "when", "pressDateBox", "pressTimeBox", "calendarMonth", "hasCalendar",
		"pressCalendar", "timeOptions", "pressTimeOption", "pressShareButton", "pressSchedule", "zone", "scheduledNotice",
		"pressButton", "pressManage", "hasDropdownItem", "pressDropdownItem", "articles", "pressMore", "pressTab",
		"scrollArticles", "rowIndex", "pressRowLink", "pressRowMenu", "hasRowMenuItem", "pressRowMenuItem", "rowAsks", "rowAnswered", "pressRowAsk",
		"pressRowTitle", "modals"} {
		if !regexp.MustCompile(`(?m)^    (async )?` + fn + `[(:]`).MatchString(linkedinJS) {
			t.Errorf("linkedin.js has no %s", fn)
		}
	}
}
