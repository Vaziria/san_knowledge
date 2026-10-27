package page

import (
	"sync"
	"time"
)

// Kept keeps the tabs of chats left open for their next ask (the user,
// 2026-10-27: "stay open and use same tab, and if there is no activity in 5
// minute, close it"): a tower's Remote Shell asks a chat again and again,
// and a tab opened and closed for each ask flickered. Each tab is kept by
// its chat, and closed once it has gone its idle time without an ask. They
// live as long as the process that opened them: a stopped san_remote leaves
// its kept tabs open.
type Kept struct {
	mu   sync.Mutex
	tabs map[string]*keptTab
}

type keptTab struct {
	s     *Session
	timer *time.Timer
}

// Take gives the session kept for key, nil when there is none; it is no
// longer kept, nor closed when its idle time ends.
func (k *Kept) Take(key string) *Session {
	k.mu.Lock()
	defer k.mu.Unlock()
	t := k.tabs[key]
	if t == nil {
		return nil
	}
	delete(k.tabs, key)
	t.timer.Stop()
	return t.s
}

// Keep keeps s for key, and closes its tab after idle unless it is taken
// before. A tab kept for key before is closed.
func (k *Kept) Keep(key string, s *Session, idle time.Duration) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.tabs == nil {
		k.tabs = make(map[string]*keptTab)
	}
	if old := k.tabs[key]; old != nil && old.s.Tab != s.Tab {
		old.timer.Stop()
		go old.s.Close()
	}
	t := &keptTab{s: s}
	t.timer = time.AfterFunc(idle, func() {
		k.mu.Lock()
		mine := k.tabs[key] == t
		if mine {
			delete(k.tabs, key)
		}
		k.mu.Unlock()
		if mine {
			s.Log("closing the chat's tab, unused for " + idle.String())
			s.Close()
		}
	})
	k.tabs[key] = t
}
