package telegram

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

//go:embed telegram.js
var telegramJS string

var site = page.Site{Name: "telegram", Global: "__sanTelegram", JS: telegramJS, ErrPage: ErrPage}

const (
	homeURL = "https://web.telegram.org/k/"
	// botFather is @BotFather's user id. The search also finds look-alikes
	// ("@BotFather", a bot named Botfagher_bot, 2026-10-27).
	botFather = "93372553"
)

// Service drives Telegram Web in the browser. Its calls run one at a time,
// since each one works in a tab of the user's browser. The CLI, the MCP
// server and the API each run one, in their own process, through
// page.Remote.
type Service struct {
	B page.Browser
	// Progress gets each step as it starts, such as "asking @BotFather for a
	// new bot".
	Progress func(step string)

	once sync.Once
	turn chan struct{}
}

func (s *Service) begin(ctx context.Context) (done func(), err error) {
	s.once.Do(func() { s.turn = make(chan struct{}, 1) })
	select {
	case s.turn <- struct{}{}:
		return func() { <-s.turn }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// run opens a shown tab at Telegram Web, waits for its chat list, and runs
// f in it. A page error comes with a screenshot.
func (s *Service) run(ctx context.Context, browser string, f func(t *session) error) error {
	done, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	p, err := page.Open(ctx, s.B, site, browser, homeURL, true, s.Progress)
	if err != nil {
		return err
	}
	t := &session{p}
	defer t.Close()
	if err := t.home(ctx); err != nil {
		return t.Fail(err)
	}
	return t.Fail(f(t))
}

// session is the one tab a call works in.
type session struct{ *page.Session }

// state is what telegram.js state() reports.
type state struct {
	URL       string `json:"url"`
	Lang      string `json:"lang"`
	Visible   bool   `json:"visible"`
	Auth      bool   `json:"auth"`
	Signin    bool   `json:"signin"`
	Ready     bool   `json:"ready"`
	LeftPage  string `json:"leftPage"`
	RightPage string `json:"rightPage"`
	Chat      string `json:"chat"`
	ChatTitle string `json:"chatTitle"`
	Input     bool   `json:"input"`
	Menu      bool   `json:"menu"`
	Popup     string `json:"popup"`
	Toast     string `json:"toast"`
}

// row is a chat row of a list: the chat list, the search's results, a
// group's members, Add Members.
type row struct {
	Peer  string `json:"peer"`
	Title string `json:"title"`
}

// msg is a message of the open chat.
type msg struct {
	Mid  float64 `json:"mid"`
	Out  bool    `json:"out"`
	Text string  `json:"text"`
}

// popupView is what telegram.js popupView() reports.
type popupView struct {
	Title   string   `json:"title"`
	Text    string   `json:"text"`
	Buttons []string `json:"buttons"`
	Checks  []struct {
		Label   string `json:"label"`
		Checked bool   `json:"checked"`
	} `json:"checks"`
}

func (v *popupView) check(label string) (found, checked bool) {
	for _, c := range v.Checks {
		if strings.EqualFold(c.Label, label) {
			return true, c.Checked
		}
	}
	return false, false
}

func (t *session) state(ctx context.Context) (state, error) {
	var st state
	err := t.Call(ctx, &st, "state")
	return st, err
}

// waitState waits until the page's state passes ok. A call that lands while
// a page loads can be refused by the browser; that is only a reason to ask
// again.
func (t *session) waitState(ctx context.Context, what string, timeout time.Duration, ok func(state) bool) (state, error) {
	var st state
	err := t.Until(ctx, what, timeout, func() (bool, error) {
		var err error
		st, err = t.state(ctx)
		if err != nil && ctx.Err() == nil && connect.CodeOf(err) == connect.CodeAborted {
			return false, nil
		}
		return err == nil && ok(st), err
	})
	return st, err
}

// home waits for Telegram Web's chat list, signed in and in English, then
// looks at it for a moment, as a person does.
func (t *session) home(ctx context.Context) error {
	t.Log("opening Telegram Web")
	if err := t.Front(ctx); err != nil {
		return err
	}
	st, err := t.waitState(ctx, "Telegram Web's chat list", time.Minute, func(st state) bool {
		return st.Ready || st.Signin || (st.URL != "" && !st.Auth)
	})
	if err != nil {
		return err
	}
	if !st.Ready {
		return fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
	}
	if st.Lang != "" && !strings.HasPrefix(st.Lang, "en") {
		return fmt.Errorf("%w: it is %q; switch it in Telegram Web's Settings, Language", ErrLanguage, st.Lang)
	}
	t.Log("looking at the chat list for a moment")
	return t.Look(ctx, 2*time.Second)
}

// main runs code in the page's own world, where Telegram Web's managers
// are: only to read what the page does not show, such as a user's
// username.
func (t *session) main(ctx context.Context, out any, code string) error {
	return t.CallMain(ctx, 20*time.Second, out, "const m = window.rootScope && window.rootScope.managers;\nif (!m) return null;\n"+code)
}

// usernames reads the usernames of users (and bots) by their ids: "" for
// one without.
func (t *session) usernames(ctx context.Context, peers []string) (map[string]string, error) {
	if len(peers) == 0 {
		return map[string]string{}, nil
	}
	ids, _ := json.Marshal(peers)
	var out map[string]string
	err := t.main(ctx, &out, fmt.Sprintf(`const out = {};
for (const id of %s) {
  try { out[id] = (await m.appPeersManager.getPeerUsername(+id)) || ''; } catch (e) { out[id] = ''; }
}
return out;`, ids))
	if err == nil && out == nil {
		err = fmt.Errorf("%w: Telegram Web has not started", ErrPage)
	}
	return out, err
}

// chatInfo is what the page knows of a group chat.
type chatInfo struct {
	Title string `json:"title"`
	// Group: a group (basic or supergroup), not a channel.
	Group bool `json:"group"`
	// Member: the account is in it.
	Member  bool `json:"member"`
	Creator bool `json:"creator"`
}

// chats reads what the page knows of group chats by their ids
// ("-5012345678"); a chat it does not know is missing.
func (t *session) chats(ctx context.Context, peers []string) (map[string]chatInfo, error) {
	if len(peers) == 0 {
		return map[string]chatInfo{}, nil
	}
	ids, _ := json.Marshal(peers)
	var out map[string]chatInfo
	err := t.main(ctx, &out, fmt.Sprintf(`const out = {};
for (const id of %s) {
  try {
    const c = await m.appChatsManager.getChat(-id);
    if (!c || !c.title) continue;
    const f = c.pFlags || {};
    out[id] = {
      title: c.title,
      group: c._ === 'chat' || (c._ === 'channel' && !!f.megagroup),
      member: (c._ === 'chat' || c._ === 'channel') && !f.left && !f.kicked && !f.deactivated,
      creator: !!f.creator,
    };
  } catch (e) {}
}
return out;`, ids))
	if err == nil && out == nil {
		err = fmt.Errorf("%w: Telegram Web has not started", ErrPage)
	}
	return out, err
}

func peers(rows []row) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Peer)
	}
	return out
}

func hasPeer(rows []row, peer string) bool {
	return slices.ContainsFunc(rows, func(r row) bool { return r.Peer == peer })
}

// leaveSearch closes the chat list's search when it is open.
func (t *session) leaveSearch(ctx context.Context) error {
	st, err := t.state(ctx)
	if err != nil || st.LeftPage != "search" {
		return err
	}
	if err := t.Press(ctx, "the search's Back button", "closeSearch"); err != nil {
		return err
	}
	_, err = t.waitState(ctx, "the chat list", 10*time.Second, func(st state) bool { return st.LeftPage == "main" })
	return err
}

// search types q in the chat list's search and waits until its results
// pass ok, or timeout. It returns the results it saw last.
func (t *session) search(ctx context.Context, q string, timeout time.Duration, ok func([]row) (bool, error)) ([]row, error) {
	t.Log("searching for " + q)
	if err := t.Press(ctx, "the search box", "search", q); err != nil {
		return nil, err
	}
	var rows []row
	err := t.Until(ctx, "the search's results for "+q, timeout, func() (bool, error) {
		st, err := t.state(ctx)
		if err != nil || st.LeftPage != "search" {
			return false, err
		}
		if err := t.Call(ctx, &rows, "rows"); err != nil {
			return false, err
		}
		return ok(rows)
	})
	return rows, err
}

// openChat opens the chat with peer: from the chat list when it shows it,
// else from the search's results for query.
func (t *session) openChat(ctx context.Context, peer, query string) error {
	var rows []row
	if err := t.Call(ctx, &rows, "rows"); err != nil {
		return err
	}
	if !hasPeer(rows, peer) {
		if _, err := t.search(ctx, query, 15*time.Second, func(rows []row) (bool, error) { return hasPeer(rows, peer), nil }); err != nil {
			if errors.Is(err, ErrPage) {
				return fmt.Errorf("%w: the search does not find %s", ErrNotFound, query)
			}
			return err
		}
	}
	if err := t.Press(ctx, "the chat's row", "openRow", peer); err != nil {
		return err
	}
	if _, err := t.waitState(ctx, "the chat to open", 15*time.Second, func(st state) bool { return st.Chat == peer }); err != nil {
		return err
	}
	return t.leaveSearch(ctx)
}

// ---- status --------------------------------------------------------------------------

// Status says whether the browser is signed in to Telegram Web, and as
// whom. Like the other calls it brings Chrome to the front: in a hidden tab
// Telegram Web does not start (its account id stayed 0, and reading the
// account never returned, 2026-10-27).
func (s *Service) Status(ctx context.Context, browser string) (*Status, error) {
	st := &Status{}
	err := s.run(ctx, browser, func(t *session) error {
		var me *struct {
			Name     string `json:"name"`
			Username string `json:"username"`
		}
		if err := t.main(ctx, &me, `const u = await m.appUsersManager.getSelf();
if (!u) return null;
const active = (u.usernames || []).find((x) => x.pFlags && x.pFlags.active);
return {name: [u.first_name, u.last_name].filter(Boolean).join(' '), username: u.username || (active ? active.username : '')};`); err != nil {
			return err
		}
		st.SignedIn = true
		if me != nil {
			st.Name, st.Username = me.Name, me.Username
		}
		return nil
	})
	if errors.Is(err, ErrNotSignedIn) {
		return &Status{}, nil
	}
	if err != nil {
		return nil, err
	}
	return st, nil
}

// ---- @BotFather ----------------------------------------------------------------------

// CreateBot asks @BotFather for a new bot, /newbot, giving its name and
// username, and returns it with its token. @BotFather's refusal (a username
// taken or not ending in bot, too many bots) is ErrRefused, with what it
// said; its conversation is then cancelled.
func (s *Service) CreateBot(ctx context.Context, browser, name, user string) (*Bot, error) {
	user, err := CheckBot(name, user)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	var b *Bot
	err = s.run(ctx, browser, func(t *session) error {
		b, err = t.createBot(ctx, name, user)
		return err
	})
	return b, err
}

// DeleteBot deletes one of the account's bots through @BotFather,
// /deletebot, answering its confirmation. A bot that is not the account's
// is ErrNotFound. It cannot be undone.
func (s *Service) DeleteBot(ctx context.Context, browser, user string) error {
	user, err := ParseUsername(user)
	if err != nil {
		return err
	}
	return s.run(ctx, browser, func(t *session) error { return t.deleteBot(ctx, user) })
}

// botFather opens the chat with @BotFather, the real one.
func (t *session) botFather(ctx context.Context) error {
	if err := t.openChat(ctx, botFather, "BotFather"); err != nil {
		return err
	}
	names, err := t.usernames(ctx, []string{botFather})
	if err != nil {
		return err
	}
	if !strings.EqualFold(names[botFather], "BotFather") {
		return fmt.Errorf("%w: chat %s is @%s, not @BotFather", ErrPage, botFather, names[botFather])
	}
	_, err = t.waitState(ctx, "the message box", 10*time.Second, func(st state) bool { return st.Input })
	return err
}

func (t *session) createBot(ctx context.Context, name, user string) (*Bot, error) {
	if err := t.botFather(ctx); err != nil {
		return nil, err
	}
	t.Log("asking @BotFather for a new bot")
	r, err := t.say(ctx, "/newbot")
	if err != nil {
		return nil, err
	}
	if !strings.Contains(strings.ToLower(r), "choose a name") {
		return nil, fmt.Errorf("%w: @BotFather: %s", ErrRefused, short(r))
	}
	t.Log("naming it " + name)
	if r, err = t.say(ctx, name); err != nil {
		return nil, err
	}
	if !strings.Contains(strings.ToLower(r), "username") {
		t.cancel(ctx)
		return nil, fmt.Errorf("%w: @BotFather: %s", ErrRefused, short(r))
	}
	t.Log("giving it the username " + user)
	if r, err = t.say(ctx, user); err != nil {
		return nil, err
	}
	tok, made, ok := madeBot(r)
	if !ok {
		t.cancel(ctx)
		return nil, fmt.Errorf("%w: @BotFather: %s", ErrRefused, short(r))
	}
	if made == "" {
		made = user
	}
	// Never the token: the log is shown.
	t.Log("@BotFather made @" + made)
	return &Bot{Name: name, Username: made, Token: tok, Link: "https://t.me/" + made}, nil
}

func (t *session) deleteBot(ctx context.Context, user string) error {
	if err := t.botFather(ctx); err != nil {
		return err
	}
	t.Log("asking @BotFather to delete @" + user)
	r, err := t.say(ctx, "/deletebot")
	if err != nil {
		return err
	}
	switch low := strings.ToLower(r); {
	case strings.Contains(low, "no bots"):
		return fmt.Errorf("%w: the account has no bots (@BotFather: %s)", ErrNotFound, short(r))
	case !strings.Contains(low, "choose a bot"):
		return fmt.Errorf("%w: @BotFather: %s", ErrRefused, short(r))
	}
	if r, err = t.say(ctx, "@"+user); err != nil {
		return err
	}
	m := confirmPhrase.FindStringSubmatch(r)
	if m == nil {
		t.cancel(ctx)
		if strings.Contains(strings.ToLower(r), "invalid bot") {
			return fmt.Errorf("%w: @%s is not one of the account's bots", ErrNotFound, user)
		}
		return fmt.Errorf("%w: @BotFather: %s", ErrRefused, short(r))
	}
	t.Log("confirming: " + m[1])
	if r, err = t.say(ctx, m[1]); err != nil {
		return err
	}
	if !strings.Contains(strings.ToLower(r), "done") {
		return fmt.Errorf("%w: @BotFather: %s", ErrRefused, short(r))
	}
	t.Log("@BotFather deleted @" + user)
	return nil
}

// cancel ends a conversation with @BotFather that went wrong, so the next
// command starts clean.
func (t *session) cancel(ctx context.Context) {
	if _, err := t.say(ctx, "/cancel"); err != nil {
		t.Log("could not send /cancel: " + err.Error())
	}
}

// say sends text in the open chat and returns the reply: the messages that
// come after it, joined once a moment has passed with no more.
func (t *session) say(ctx context.Context, text string) (string, error) {
	var last float64
	if err := t.Call(ctx, &last, "lastMid"); err != nil {
		return "", err
	}
	if err := t.Press(ctx, "the message box", "typeMessage", text); err != nil {
		return "", err
	}
	if err := t.Until(ctx, "the message to be typed", 10*time.Second, func() (bool, error) {
		var draft string
		var can bool
		if err := t.Call(ctx, &draft, "draft"); err != nil {
			return false, err
		}
		if err := t.Call(ctx, &can, "canSend"); err != nil {
			return false, err
		}
		return draft == text && can, nil
	}); err != nil {
		return "", err
	}
	if err := t.Press(ctx, "Send", "pressSend"); err != nil {
		return "", err
	}
	var sent float64
	if err := t.Until(ctx, "the message to be sent", 15*time.Second, func() (bool, error) {
		var ms []msg
		if err := t.Call(ctx, &ms, "after", last); err != nil {
			return false, err
		}
		for _, m := range ms {
			if m.Out && strings.TrimSpace(m.Text) == text {
				sent = m.Mid
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return "", err
	}
	replies := func() ([]string, error) {
		var ms []msg
		if err := t.Call(ctx, &ms, "after", sent); err != nil {
			return nil, err
		}
		var out []string
		for _, m := range ms {
			if !m.Out {
				out = append(out, m.Text)
			}
		}
		return out, nil
	}
	var got []string
	if err := t.Until(ctx, "a reply", 30*time.Second, func() (bool, error) {
		var err error
		got, err = replies()
		return len(got) > 0, err
	}); err != nil {
		return "", err
	}
	// A reply can come in more than one message.
	for {
		if err := page.Sleep(ctx, 1500*time.Millisecond); err != nil {
			return "", err
		}
		more, err := replies()
		if err != nil {
			return "", err
		}
		if len(more) == len(got) {
			return strings.Join(got, "\n\n"), nil
		}
		got = more
	}
}

// ---- groups --------------------------------------------------------------------------

// CreateGroup makes a group of the account's with title and no other
// members yet: New Chats, New Group, Next past Add Members, its name, Next.
func (s *Service) CreateGroup(ctx context.Context, browser, title string) (*Group, error) {
	if err := CheckTitle(title); err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	var g *Group
	err := s.run(ctx, browser, func(t *session) error {
		var err error
		g, err = t.createGroup(ctx, title)
		return err
	})
	return g, err
}

// DeleteGroup deletes a group of the account's for all its members, from
// the chat's More menu, Delete Group, with "Delete for all members" ticked. group
// is its id ("-5012345678") or its title. A group the account may only
// leave is ErrRefused. It cannot be undone.
func (s *Service) DeleteGroup(ctx context.Context, browser, group string) (*Group, error) {
	var g *Group
	err := s.run(ctx, browser, func(t *session) error {
		var err error
		g, err = t.deleteGroup(ctx, group)
		return err
	})
	return g, err
}

// AddMember adds a user, by username, to a group of the account's, from the
// group's profile: Add Members, the user, Next, Add. A user already in it
// is left as they are. A user whose privacy settings keep them out of
// groups is ErrRefused.
func (s *Service) AddMember(ctx context.Context, browser, group, user string) (*Group, *Member, error) {
	user, err := ParseUsername(user)
	if err != nil {
		return nil, nil, err
	}
	var g *Group
	var m *Member
	err = s.run(ctx, browser, func(t *session) error {
		var err error
		g, m, err = t.addMember(ctx, group, user)
		return err
	})
	return g, m, err
}

// RemoveMember removes a user, by username, from a group of the account's:
// in the group's profile, the member's right-click menu, Remove from group.
// A user who is not a member is ErrNotFound; a group the account may not
// remove members from is ErrRefused.
func (s *Service) RemoveMember(ctx context.Context, browser, group, user string) (*Group, *Member, error) {
	user, err := ParseUsername(user)
	if err != nil {
		return nil, nil, err
	}
	var g *Group
	var m *Member
	err = s.run(ctx, browser, func(t *session) error {
		var err error
		g, m, err = t.removeMember(ctx, group, user)
		return err
	})
	return g, m, err
}

func (t *session) createGroup(ctx context.Context, title string) (*Group, error) {
	t.Log("opening New Chats, then New Group")
	var err error
	for try := 1; try <= 3; try++ {
		if err = t.Press(ctx, "New Chats", "pressNewMenu"); err != nil {
			return nil, err
		}
		if _, err = t.waitState(ctx, "the New Chats menu", 5*time.Second, func(st state) bool { return st.Menu }); err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	if err := t.Press(ctx, "New Group", "pressMenuItem", "New Group"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "Add Members", 10*time.Second, func(st state) bool { return st.LeftPage == "add-members" }); err != nil {
		return nil, err
	}
	t.Log("going on without members")
	if err := t.Press(ctx, "Next", "pressNext", "left"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the group's name", 10*time.Second, func(st state) bool { return st.LeftPage == "new-group" }); err != nil {
		return nil, err
	}
	t.Log("naming it " + title)
	if err := t.Press(ctx, "the group name box", "typeGroupName", title); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "the name to be typed", 10*time.Second, func() (bool, error) {
		var name string
		err := t.Call(ctx, &name, "groupName")
		return err == nil && name == title, err
	}); err != nil {
		return nil, err
	}
	t.Log("creating the group")
	if err := t.Press(ctx, "Next", "pressNext", "left"); err != nil {
		return nil, err
	}
	st, err := t.waitState(ctx, "the new group's chat", 30*time.Second, func(st state) bool {
		return IsGroupID(st.Chat) && st.ChatTitle == title && st.LeftPage == "main"
	})
	if err != nil {
		return nil, err
	}
	t.Log("made group " + st.Chat)
	return &Group{ID: st.Chat, Title: title}, nil
}

// findGroup opens a group of the account's: by its id, or by its title,
// which must be one group's.
func (t *session) findGroup(ctx context.Context, group string) (*Group, error) {
	group = strings.TrimSpace(group)
	g := &Group{}
	if IsGroupID(group) {
		info, err := t.chats(ctx, []string{group})
		if err != nil {
			return nil, err
		}
		c, ok := info[group]
		if !ok || !c.Member {
			return nil, fmt.Errorf("%w: the account is in no group %s", ErrNotFound, group)
		}
		if !c.Group {
			return nil, fmt.Errorf("%w: %s (%s) is a channel, not a group", ErrInvalid, group, c.Title)
		}
		g.ID, g.Title = group, c.Title
		return g, t.openChat(ctx, g.ID, g.Title)
	}
	if err := CheckTitle(group); err != nil {
		return nil, err
	}
	g.Title = group
	// The account's groups of that title: a public group the search finds
	// that the account is not in does not count.
	mine := func(rows []row) ([]string, error) {
		var ids []string
		for _, r := range rows {
			if r.Title == group && IsGroupID(r.Peer) {
				ids = append(ids, r.Peer)
			}
		}
		info, err := t.chats(ctx, ids)
		if err != nil {
			return nil, err
		}
		return slices.DeleteFunc(ids, func(id string) bool { c, ok := info[id]; return !ok || !c.Group || !c.Member }), nil
	}
	var found []string
	_, err := t.search(ctx, group, 15*time.Second, func(rows []row) (bool, error) {
		var err error
		found, err = mine(rows)
		return len(found) > 0, err
	})
	switch {
	case errors.Is(err, ErrPage):
		return nil, fmt.Errorf("%w: the account is in no group titled %q", ErrNotFound, group)
	case err != nil:
		return nil, err
	}
	// The search's results come in parts: a moment more for a second group
	// of the same title.
	if err := page.Sleep(ctx, 2*time.Second); err != nil {
		return nil, err
	}
	var rows []row
	if err := t.Call(ctx, &rows, "rows"); err != nil {
		return nil, err
	}
	if more, err := mine(rows); err != nil {
		return nil, err
	} else if len(more) > len(found) {
		found = more
	}
	if len(found) > 1 {
		return nil, fmt.Errorf("%w: %d groups are titled %q (%s): give its id", ErrAmbiguous, len(found), group, strings.Join(found, ", "))
	}
	g.ID = found[0]
	return g, t.openChat(ctx, g.ID, g.Title)
}

func (t *session) deleteGroup(ctx context.Context, group string) (*Group, error) {
	g, err := t.findGroup(ctx, group)
	if err != nil {
		return nil, err
	}
	t.Log("opening the chat's menu, then Delete Group")
	if err := t.Press(ctx, "the chat's More button", "pressChatMenu"); err != nil {
		return g, err
	}
	var items []string
	if err := t.Until(ctx, "the chat's menu", 5*time.Second, func() (bool, error) {
		err := t.Call(ctx, &items, "menuItems")
		return err == nil && len(items) > 0, err
	}); err != nil {
		return g, err
	}
	// The menu names its last item as Telegram Web's getDeleteButtonText
	// does: Delete Group for a group the account may delete, else Leave
	// Group (2026-10-27).
	const deleteItem = "Delete Group"
	if !slices.Contains(items, deleteItem) {
		_ = t.Call(ctx, nil, "closeMenu")
		if slices.Contains(items, "Leave Group") {
			return g, fmt.Errorf("%w: Telegram offers only to leave %q: the account is not its owner", ErrRefused, g.Title)
		}
		return g, fmt.Errorf("%w: the chat's menu has no %s (%s)", ErrPage, deleteItem, strings.Join(items, ", "))
	}
	if err := t.Press(ctx, deleteItem, "pressMenuItem", deleteItem); err != nil {
		return g, err
	}
	if _, err := t.waitState(ctx, "the delete popup", 10*time.Second, func(st state) bool { return strings.Contains(st.Popup, "popup-delete-chat") }); err != nil {
		return g, err
	}
	var v popupView
	if err := t.Call(ctx, &v, "popupView"); err != nil {
		return g, err
	}
	const forAll = "Delete for all members"
	found, checked := v.check(forAll)
	if !found {
		_ = t.Call(ctx, nil, "closePopup")
		return g, fmt.Errorf("%w: Telegram offers only to leave %q (%s): the account is not its owner", ErrRefused, g.Title, v.Title)
	}
	if !checked {
		t.Log("ticking " + forAll)
		if err := t.Press(ctx, forAll, "tickPopup", forAll); err != nil {
			return g, err
		}
		if err := t.Until(ctx, forAll+" to be ticked", 5*time.Second, func() (bool, error) {
			if err := t.Call(ctx, &v, "popupView"); err != nil {
				return false, err
			}
			_, checked := v.check(forAll)
			return checked, nil
		}); err != nil {
			return g, err
		}
	}
	t.Log("deleting the group for all its members")
	if err := t.Press(ctx, "Delete group", "pressPopupButton", "Delete group"); err != nil {
		return g, err
	}
	// Deleted: Telegram marks the chat deactivated, so the account is no
	// longer in it. The screen is no sure sign: in the live test the group
	// was gone from the list while the tab still showed the chat
	// (2026-10-27).
	if err := t.Until(ctx, "the group to be deleted", 20*time.Second, func() (bool, error) {
		info, err := t.chats(ctx, []string{g.ID})
		c, ok := info[g.ID]
		return err == nil && (!ok || !c.Member), err
	}); err != nil {
		return g, err
	}
	t.Log("deleted group " + g.ID)
	return g, nil
}

// openMembers opens the open group's profile at its members.
func (t *session) openMembers(ctx context.Context) error {
	st, err := t.state(ctx)
	if err != nil {
		return err
	}
	if st.RightPage != "profile" {
		t.Log("opening the group's profile")
		if err := t.Press(ctx, "the chat's title", "pressProfile"); err != nil {
			return err
		}
		if _, err := t.waitState(ctx, "the group's profile", 10*time.Second, func(st state) bool { return st.RightPage == "profile" }); err != nil {
			return err
		}
	}
	var tabs []string
	if err := t.Until(ctx, "the profile's tabs", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &tabs, "tabs")
		return err == nil && len(tabs) > 0, err
	}); err != nil {
		return err
	}
	if slices.Contains(tabs, "Members") {
		if err := t.Press(ctx, "Members", "pressTab", "Members"); err != nil {
			return err
		}
	}
	// The account is a member: the list is never empty once it has loaded.
	return t.Until(ctx, "the members list", 15*time.Second, func() (bool, error) {
		var rows []row
		err := t.Call(ctx, &rows, "members")
		return err == nil && len(rows) > 0, err
	})
}

// member finds the member of the open profile with username user,
// scrolling for the members a big group loads as it scrolls.
func (t *session) member(ctx context.Context, user string) (*Member, error) {
	seen := 0
	for range 20 {
		var rows []row
		if err := t.Call(ctx, &rows, "members"); err != nil {
			return nil, err
		}
		names, err := t.usernames(ctx, peers(rows))
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if strings.EqualFold(names[r.Peer], user) {
				return &Member{ID: r.Peer, Name: r.Title, Username: names[r.Peer]}, nil
			}
		}
		if len(rows) == seen {
			return nil, nil
		}
		seen = len(rows)
		if err := t.Call(ctx, nil, "moreMembers"); err != nil {
			return nil, err
		}
		if err := page.Sleep(ctx, 1500*time.Millisecond); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (t *session) addMember(ctx context.Context, group, user string) (*Group, *Member, error) {
	g, err := t.findGroup(ctx, group)
	if err != nil {
		return nil, nil, err
	}
	if err := t.openMembers(ctx); err != nil {
		return g, nil, err
	}
	if m, err := t.member(ctx, user); err != nil || m != nil {
		if m != nil {
			t.Log("@" + user + " is already a member")
		}
		return g, m, err
	}
	t.Log("opening Add Members")
	if err := t.Press(ctx, "Add Members", "pressAddMembers"); err != nil {
		return g, nil, err
	}
	if _, err := t.waitState(ctx, "Add Members", 10*time.Second, func(st state) bool { return st.RightPage == "add-members" }); err != nil {
		return g, nil, err
	}
	t.Log("searching for @" + user)
	if err := t.Press(ctx, "the search box", "pickerSearch", "right", user); err != nil {
		return g, nil, err
	}
	var m *Member
	err = t.Until(ctx, "@"+user+" in the search's results", 15*time.Second, func() (bool, error) {
		var rows []row
		if err := t.Call(ctx, &rows, "pickerRows", "right"); err != nil {
			return false, err
		}
		names, err := t.usernames(ctx, peers(rows))
		if err != nil {
			return false, err
		}
		for _, r := range rows {
			if strings.EqualFold(names[r.Peer], user) {
				m = &Member{ID: r.Peer, Name: r.Title, Username: names[r.Peer]}
				return true, nil
			}
		}
		return false, nil
	})
	if errors.Is(err, ErrPage) {
		return g, nil, fmt.Errorf("%w: Telegram finds no user @%s", ErrNotFound, user)
	}
	if err != nil {
		return g, nil, err
	}
	if err := t.Press(ctx, m.Name, "pick", "right", m.ID); err != nil {
		return g, m, err
	}
	if err := t.Until(ctx, m.Name+" to be ticked", 5*time.Second, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "picked", "right", m.ID)
		return ok, err
	}); err != nil {
		return g, m, err
	}
	if err := t.Press(ctx, "Next", "pressNext", "right"); err != nil {
		return g, m, err
	}
	if _, err := t.waitState(ctx, "the Add popup", 10*time.Second, func(st state) bool { return strings.Contains(st.Popup, "popup-add-members") }); err != nil {
		return g, m, err
	}
	t.Log("adding @" + m.Username + " to " + g.Title)
	if err := t.Press(ctx, "Add", "pressPopupButton", "Add"); err != nil {
		return g, m, err
	}
	// Added: the profile shows the member. Refused: a toast, or a popup
	// offering an invite link instead.
	var refused string
	err = t.Until(ctx, "@"+user+" to be added", 20*time.Second, func() (bool, error) {
		st, err := t.state(ctx)
		if err != nil {
			return false, err
		}
		if strings.Contains(strings.ToLower(st.Toast), "privacy") || strings.Contains(st.Toast, "can't") {
			refused = st.Toast
			return true, nil
		}
		if st.Popup != "" && !strings.Contains(st.Popup, "popup-add-members") {
			var v popupView
			if err := t.Call(ctx, &v, "popupView"); err != nil {
				return false, err
			}
			refused = strings.TrimSpace(v.Title + ": " + v.Text)
			_ = t.Call(ctx, nil, "closePopup")
			return true, nil
		}
		if st.RightPage != "profile" {
			return false, nil
		}
		var rows []row
		if err := t.Call(ctx, &rows, "members"); err != nil {
			return false, err
		}
		return hasPeer(rows, m.ID), nil
	})
	if err != nil {
		return g, m, err
	}
	if refused != "" {
		return g, m, fmt.Errorf("%w: Telegram did not add @%s to %q: %s", ErrRefused, m.Username, g.Title, short(refused))
	}
	t.Log("added @" + m.Username)
	_ = t.Call(ctx, nil, "closeProfile")
	return g, m, nil
}

func (t *session) removeMember(ctx context.Context, group, user string) (*Group, *Member, error) {
	g, err := t.findGroup(ctx, group)
	if err != nil {
		return nil, nil, err
	}
	if err := t.openMembers(ctx); err != nil {
		return g, nil, err
	}
	m, err := t.member(ctx, user)
	if err != nil {
		return g, nil, err
	}
	if m == nil {
		return g, nil, fmt.Errorf("%w: @%s is not a member of %q", ErrNotFound, user, g.Title)
	}
	t.Log("opening " + m.Name + "'s menu")
	if err := t.Press(ctx, m.Name+"'s row", "openMemberMenu", m.ID); err != nil {
		return g, m, err
	}
	var items []string
	if err := t.Until(ctx, "the member's menu", 5*time.Second, func() (bool, error) {
		err := t.Call(ctx, &items, "menuItems")
		return err == nil && len(items) > 0, err
	}); err != nil {
		return g, m, err
	}
	const remove = "Remove from group"
	if !slices.Contains(items, remove) {
		_ = t.Call(ctx, nil, "closeMenu")
		return g, m, fmt.Errorf("%w: Telegram offers no %s for @%s in %q: the account may not remove them", ErrRefused, remove, m.Username, g.Title)
	}
	t.Log("removing @" + m.Username + " from " + g.Title)
	if err := t.Press(ctx, remove, "pressMenuItem", remove); err != nil {
		return g, m, err
	}
	if err := t.Until(ctx, "@"+user+" to leave the members list", 15*time.Second, func() (bool, error) {
		var rows []row
		err := t.Call(ctx, &rows, "members")
		return err == nil && !hasPeer(rows, m.ID), err
	}); err != nil {
		return g, m, err
	}
	t.Log("removed @" + m.Username)
	_ = t.Call(ctx, nil, "closeProfile")
	return g, m, nil
}
