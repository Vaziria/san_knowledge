package kb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const launchDoc = "# Launch\n\nIntro.\n\n## Plan\n\n" +
	"- [ ] Write the **article** #writing #Medium\n" + // 7
	"  - [x] outline\n" + //                              8
	"  - [ ] draft\n" + //                                9
	"    - [ ] intro\n" + //                             10
	"  - a plain note under it\n" + //                   11
	"- [>] Record [video](../v.md) #video\n" + //        12
	"- [/] Build board\n" + //                          13
	"- [?] Review copy\n" + //                          14
	"- [x] Ship zip\n" + //                             15
	"- [-] dropped\n" + //                              16
	"1. [ ] numbered item\n" + //                       17
	"Some paragraph.\n" + //                            18
	"  - [ ] after a paragraph\n\n" + //                19
	"```md\n- [ ] in a fence\n```\n\n" + //            21-23
	"## Later\n\n" + //                                 25
	"* [X] uppercase done\n" + //                       27
	"- [ ] Issue #12 stays in the title\n"

func todoTitles(todos []Todo) []string {
	var out []string
	for _, t := range todos {
		out = append(out, string(t.Status)+":"+t.Title)
	}
	return out
}

func TestParseTodos(t *testing.T) {
	todos := ParseTodos(ParseMarkdown("docs/launch.md", []byte(launchDoc)))
	got := strings.Join(todoTitles(todos), "|")
	want := "backlog:Write the article|ready:Record video|in_progress:Build board|review:Review copy|done:Ship zip|" +
		"backlog:numbered item|backlog:after a paragraph|done:uppercase done|backlog:Issue #12 stays in the title"
	if got != want {
		t.Fatalf("cards:\n got %s\nwant %s", got, want)
	}
	w := todos[0]
	if w.Kind != TodoCheckbox || w.Line != 7 || w.Key != "docs/launch.md#plan" || w.Section != "Plan" || w.Doc != "Launch" ||
		strings.Join(w.Labels, ",") != "writing,medium" || w.Raw != "- [ ] Write the **article** #writing #Medium" {
		t.Fatalf("first card: %+v", w)
	}
	if len(w.Subtasks) != 3 || w.Done != 1 || w.Subtasks[0].Title != "outline" || !w.Subtasks[0].Done ||
		w.Subtasks[2].Depth != 1 || w.Subtasks[2].Line != 10 {
		t.Fatalf("subtasks: %+v (done %d)", w.Subtasks, w.Done)
	}
	if todos[1].Labels[0] != "video" || len(todos[5].Subtasks) != 0 {
		t.Fatalf("labels / a paragraph closes the card: %+v %+v", todos[1], todos[5])
	}
	if todos[7].Key != "docs/launch.md#later" || len(todos[8].Labels) != 0 {
		t.Fatalf("section / no number labels: %+v %+v", todos[7], todos[8])
	}

	// A task file is one card; its folder is the column and its checkboxes the subtasks.
	task := ParseTodos(ParseMarkdown("tasks/doing/03-video.md", []byte(
		"# Make the video\n\nLabels: Video, Launch Plan\n\n## Do\n\n- [x] script\n- [ ] record\n  - [ ] b-roll\n\nAfter: 02-copy.md\n")))
	if len(task) != 1 {
		t.Fatalf("task cards: %+v", task)
	}
	tk := task[0]
	if tk.Kind != TodoTask || tk.ID != "tasks/doing/03-video.md" || tk.Status != TodoInProgress || tk.Title != "Make the video" || tk.Line != 1 ||
		strings.Join(tk.Labels, ",") != "video,launch-plan" || tk.After != "02-copy.md" || len(tk.Subtasks) != 3 || tk.Done != 1 || tk.Subtasks[2].Depth != 1 {
		t.Fatalf("task card: %+v", tk)
	}

	for loc, want := range map[string]TodoStatus{"tasks/a.md": TodoReady, "tasks/backlog/a.md": TodoBacklog, "tasks/doing/a.md": TodoInProgress,
		"tasks/blocked/a.md": TodoReview, "tasks/done/a.md": TodoDone} {
		if st, ok := TaskStatus(loc); !ok || st != want {
			t.Fatalf("TaskStatus(%s) = %s %v", loc, st, ok)
		}
	}
	for _, loc := range []string{"tasks/other/a.md", "docs/tasks/a.md", "tasks/a.txt", "tasks/done/x/a.md"} {
		if _, ok := TaskStatus(loc); ok {
			t.Fatalf("%s is not a task file", loc)
		}
	}
}

func cardByTitle(t *testing.T, s *Store, title string) Todo {
	t.Helper()
	todos, err := s.Todos()
	if err != nil {
		t.Fatal(err)
	}
	for _, td := range todos {
		if td.Title == title {
			return td
		}
	}
	t.Fatalf("no card %q in %v", title, todoTitles(todos))
	return Todo{}
}

func readDoc(t *testing.T, s *Store, loc string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(loc)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestTodoPage(t *testing.T) {
	s, _ := project(t, map[string]string{"launch.md": launchDoc})
	writeFile(t, s.Root, "knowledge_data/"+ConfigFile, `{"track": ["."]}`)
	writeFile(t, s.Root, "tasks/backlog/04-post.md", "# Post the video\n\nLabels: Video\n\n- [ ] caption\n")
	query := func(q TodoQuery) TodoPage {
		t.Helper()
		page, err := s.TodoPage(q)
		if err != nil {
			t.Fatal(err)
		}
		return page
	}
	column := func(c TodoColumn) string {
		var out []string
		for _, td := range c.Todos {
			out = append(out, td.Title)
		}
		return fmt.Sprintf("%s %d: %s", c.Status, c.Total, strings.Join(out, "|"))
	}

	// Every column, two cards each, with the column totals.
	page := query(TodoQuery{Limit: 2})
	if page.Total != 10 || len(page.Columns) != 5 {
		t.Fatalf("page: total %d, %d columns", page.Total, len(page.Columns))
	}
	if got := column(page.Columns[0]); got != "backlog 5: Write the article|numbered item" {
		t.Fatalf("first page: %s", got)
	}
	if got := column(page.Columns[4]); got != "done 2: Ship zip|uppercase done" {
		t.Fatalf("done column: %s", got)
	}
	// The next page of one column.
	page = query(TodoQuery{Status: TodoBacklog, Offset: 4, Limit: 2})
	if len(page.Columns) != 1 || column(page.Columns[0]) != "backlog 5: Post the video" {
		t.Fatalf("last page: %+v", page.Columns)
	}

	// Filters apply before paging; the board total does not change.
	for _, c := range []struct {
		q    TodoQuery
		want string
	}{
		{TodoQuery{Labels: []string{"#Video"}}, "backlog 1: Post the video,ready 1: Record video"},
		{TodoQuery{Search: "OUTLINE"}, "backlog 1: Write the article"},                    // a subtask
		{TodoQuery{Search: "#medium writ"}, "backlog 1: Write the article"},               // a label and a word
		{TodoQuery{Search: "launch.md issue"}, "backlog 1: Issue #12 stays in the title"}, // the path
		{TodoQuery{Kind: TodoTask}, "backlog 1: Post the video"},
		{TodoQuery{Search: "nothing like this"}, ""},
	} {
		page := query(c.q)
		var got []string
		for _, col := range page.Columns {
			if col.Total > 0 {
				got = append(got, column(col))
			}
		}
		if strings.Join(got, ",") != c.want || page.Total != 10 {
			t.Fatalf("%+v:\n got %s (total %d)\nwant %s", c.q, strings.Join(got, ","), page.Total, c.want)
		}
	}

	for _, q := range []TodoQuery{{Status: "later"}, {Kind: "note"}} {
		if _, err := s.TodoPage(q); err == nil {
			t.Fatalf("%+v: no error", q)
		}
	}
}

func TestEditTodo(t *testing.T) {
	s, _ := project(t, map[string]string{"launch.md": strings.ReplaceAll(launchDoc, "\n", "\r\n")})
	writeFile(t, s.Root, "knowledge_data/"+ConfigFile, `{"track": ["."]}`)
	writeFile(t, s.Root, "tasks/03-video.md", "# Make the video\n\nRecord it.\n")
	writeFile(t, s.Root, "notes/untracked.txt", "- [ ] not markdown\n")

	// Moving a checkbox rewrites its marker and keeps CRLF.
	rec := cardByTitle(t, s, "Record video")
	if _, err := s.EditTodo(TodoEdit{Op: "move", Kind: rec.Kind, Loc: rec.Loc, Line: rec.Line, Raw: rec.Raw, To: TodoInProgress}); err != nil {
		t.Fatal(err)
	}
	if doc := readDoc(t, s, "docs/launch.md"); !strings.Contains(doc, "\r\n- [/] Record [video](../v.md) #video\r\n") || strings.Count(doc, "\r\n") != strings.Count(doc, "\n") {
		t.Fatalf("marker or CRLF:\n%q", doc)
	}
	// A stale line is refused, and nothing is written.
	if _, err := s.EditTodo(TodoEdit{Op: "move", Kind: rec.Kind, Loc: rec.Loc, Line: rec.Line, Raw: rec.Raw, To: TodoDone}); !errors.Is(err, ErrTodoChanged) {
		t.Fatalf("stale raw: %v", err)
	}

	art := cardByTitle(t, s, "Write the article")
	if _, err := s.EditTodo(TodoEdit{Op: "labels", Kind: art.Kind, Loc: art.Loc, Line: art.Line, Raw: art.Raw, Labels: []string{"#Medium", "Launch Plan", "medium", "!!"}}); err != nil {
		t.Fatal(err)
	}
	art = cardByTitle(t, s, "Write the article")
	if art.Raw != "- [ ] Write the **article** #medium #launch-plan" {
		t.Fatalf("labels: %q", art.Raw)
	}
	// A new subtask follows the existing ones' indent and goes after the card's block.
	if _, err := s.EditTodo(TodoEdit{Op: "subtask", Kind: art.Kind, Loc: art.Loc, Line: art.Line, Raw: art.Raw, Text: " publish\n it "}); err != nil {
		t.Fatal(err)
	}
	art = cardByTitle(t, s, "Write the article")
	if n := len(art.Subtasks); n != 4 || art.Subtasks[3].Title != "publish it" || art.Subtasks[3].Line != 12 || art.Subtasks[3].Depth != 0 {
		t.Fatalf("subtask added: %+v", art.Subtasks)
	}
	draft := art.Subtasks[1]
	if _, err := s.EditTodo(TodoEdit{Op: "check", Kind: art.Kind, Loc: art.Loc, Line: draft.Line, Raw: draft.Raw, Done: true}); err != nil {
		t.Fatal(err)
	}
	if art = cardByTitle(t, s, "Write the article"); art.Done != 2 {
		t.Fatalf("check: done %d", art.Done)
	}
	rev := cardByTitle(t, s, "Review copy")
	if _, err := s.EditTodo(TodoEdit{Op: "subtask", Kind: rev.Kind, Loc: rev.Loc, Line: rev.Line, Raw: rev.Raw, Text: "fix typos"}); err != nil {
		t.Fatal(err)
	}
	if rev = cardByTitle(t, s, "Review copy"); len(rev.Subtasks) != 1 || !strings.HasPrefix(readDoc(t, s, "docs/launch.md")[strings.Index(readDoc(t, s, "docs/launch.md"), "- [?] Review copy"):], "- [?] Review copy\r\n  - [ ] fix typos\r\n") {
		t.Fatalf("first subtask: %+v", rev)
	}

	// Task files: labels go on a Labels: line, a first subtask opens "## Subtasks",
	// and moving changes the folder.
	if _, err := s.EditTodo(TodoEdit{Op: "labels", Kind: TodoTask, Loc: "tasks/03-video.md", Labels: []string{"video"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EditTodo(TodoEdit{Op: "subtask", Kind: TodoTask, Loc: "tasks/03-video.md", Text: "storyboard"}); err != nil {
		t.Fatal(err)
	}
	if got := readDoc(t, s, "tasks/03-video.md"); got != "# Make the video\n\nLabels: video\n\nRecord it.\n\n## Subtasks\n\n- [ ] storyboard\n" {
		t.Fatalf("task file:\n%q", got)
	}
	if _, err := s.EditTodo(TodoEdit{Op: "labels", Kind: TodoTask, Loc: "tasks/03-video.md", Labels: []string{"video", "tiktok"}}); err != nil {
		t.Fatal(err)
	}
	loc, err := s.EditTodo(TodoEdit{Op: "move", Kind: TodoTask, Loc: "tasks/03-video.md", To: TodoBacklog})
	if err != nil || loc != "tasks/backlog/03-video.md" {
		t.Fatalf("task move: %s %v", loc, err)
	}
	tk := cardByTitle(t, s, "Make the video")
	if tk.Status != TodoBacklog || strings.Join(tk.Labels, ",") != "video,tiktok" || len(tk.Subtasks) != 1 {
		t.Fatalf("moved task: %+v", tk)
	}
	if _, err := os.Stat(filepath.Join(s.Root, "tasks", "03-video.md")); !os.IsNotExist(err) {
		t.Fatalf("old task file still there: %v", err)
	}

	if _, err := s.EditTodo(TodoEdit{Op: "move", Kind: TodoCheckbox, Loc: "notes/untracked.txt", Line: 1, Raw: "- [ ] not markdown", To: TodoDone}); err == nil {
		t.Fatal("an untracked file was edited")
	}
	if _, err := s.EditTodo(TodoEdit{Op: "move", Kind: TodoTask, Loc: "docs/launch.md", To: TodoDone}); err == nil {
		t.Fatal("a doc outside tasks/ was moved as a task")
	}
}

func TestSyncCarriesMovedDocs(t *testing.T) {
	s, _ := project(t, map[string]string{"a/note.md": "# Note\n\nIntro.\n\n## Part\n\nBody.\n"})
	if _, err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	sum, part := "A note.", "The part."
	if _, err := s.Annotate("docs/a/note.md", Annotation{Summary: &sum, Keyword: []string{"note"}, Author: AuthorAI}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Annotate("docs/a/note.md#part", Annotation{Summary: &part, Author: AuthorAI}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AssignDomain("docs/a/note.md", "Notes", AuthorAI); err != nil {
		t.Fatal(err)
	}

	// Same text at a new path: everything carries over, nothing is pending.
	if err := os.Rename(filepath.Join(s.Root, "docs", "a"), filepath.Join(s.Root, "docs", "b")); err != nil {
		t.Fatal(err)
	}
	r, err := s.Sync()
	if err != nil || len(r.DocsMoved) != 1 || len(r.DocsAdded) != 0 || len(r.DocsRemoved) != 0 {
		t.Fatalf("move report: %+v %v", r, err)
	}
	d, err := s.Get("docs/b/note.md")
	if err != nil || d.Summary != sum || d.NeedsSummary || strings.Join(d.Keyword, ",") != "note" {
		t.Fatalf("moved doc: %+v %v", d, err)
	}
	if p, err := s.Get("docs/b/note.md#part"); err != nil || p.Summary != part || p.NeedsSummary {
		t.Fatalf("moved section: %+v %v", p, err)
	}
	if ds, err := s.DomainsOf("docs/b/note.md"); err != nil || len(ds) != 1 || ds[0].Title != "Notes" {
		t.Fatalf("moved domain: %v %v", ds, err)
	}
	if _, err := s.Get("docs/a/note.md"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old doc kept: %v", err)
	}

	// Moved and edited: matched by file name and title; changed text is flagged, the old summary kept.
	if err := os.MkdirAll(filepath.Join(s.Root, "docs", "c"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(s.Root, "docs", "b", "note.md"), filepath.Join(s.Root, "docs", "c", "note.md")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, s.Root, "docs/c/note.md", "# Note\n\nIntro.\n\n## Part\n\nNew body.\n")
	if r, err := s.Sync(); err != nil || len(r.DocsMoved) != 1 {
		t.Fatalf("edited move: %+v %v", r, err)
	}
	if p, err := s.Get("docs/c/note.md#part"); err != nil || p.Summary != part || !p.NeedsSummary {
		t.Fatalf("edited section: %+v %v", p, err)
	}
}
