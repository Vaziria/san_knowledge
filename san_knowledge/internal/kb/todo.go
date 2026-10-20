package kb

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The board (the view's Board mode) reads to-do items straight from the
// tracked markdown and writes changes back into it; nothing about them is
// stored in the graph. Two kinds of card:
//
//   - a checkbox item in any doc: "- [ ] text". Its marker is its column, "#word"
//     tokens in its text are labels, and checkbox items indented under it are
//     its subtasks.
//   - a task file of the shared queue in tasks/ (CLAUDE.md "Tasks"). Its folder
//     is its column, a "Labels: a, b" line holds its labels, and every checkbox
//     item in the file is a subtask.

// TasksDir is the shared task queue, relative to the project root.
const TasksDir = "tasks"

// TodoStatus is a board column.
type TodoStatus string

const (
	TodoBacklog    TodoStatus = "backlog"
	TodoReady      TodoStatus = "ready"
	TodoInProgress TodoStatus = "in_progress"
	TodoReview     TodoStatus = "review"
	TodoDone       TodoStatus = "done"
)

// TodoStatuses are the board's columns, in order.
var TodoStatuses = []TodoStatus{TodoBacklog, TodoReady, TodoInProgress, TodoReview, TodoDone}

// Card kinds.
const (
	TodoCheckbox = "checkbox"
	TodoTask     = "task"
)

var (
	// A checkbox item's marker is its column; markers outside this map ("[-]") are not to-dos.
	markerStatus = map[byte]TodoStatus{' ': TodoBacklog, '>': TodoReady, '/': TodoInProgress, '?': TodoReview, 'x': TodoDone, 'X': TodoDone}
	statusMarker = map[TodoStatus]byte{TodoBacklog: ' ', TodoReady: '>', TodoInProgress: '/', TodoReview: '?', TodoDone: 'x'}
	// A task file's folder under tasks/ is its column. Sessions only take files
	// directly in tasks/, so backlog/ parks a task until it is ready.
	folderStatus = map[string]TodoStatus{"": TodoReady, "backlog": TodoBacklog, "doing": TodoInProgress, "blocked": TodoReview, "done": TodoDone}
	statusFolder = map[TodoStatus]string{TodoReady: "", TodoBacklog: "backlog", TodoInProgress: "doing", TodoReview: "blocked", TodoDone: "done"}

	checkboxRe   = regexp.MustCompile(`^([ \t]*)([-*+]|\d{1,9}[.)])[ \t]+\[([ xX>/?])\](?:[ \t]+(.*))?$`)
	labelRe      = regexp.MustCompile(`(^|\s)#(\p{L}[\p{L}\p{N}_/-]*)`) // a letter first: "issue #12" is no label
	labelsLineRe = regexp.MustCompile(`(?i)^labels:[ \t]*(.*)$`)
	afterLineRe  = regexp.MustCompile(`(?i)^after:[ \t]*(.*)$`)
	emphasisRe   = regexp.MustCompile("\\*\\*|__|`")
)

// ErrTodoChanged means the line the board showed is no longer in the file.
var ErrTodoChanged = errors.New("the file changed since the board loaded it; reload and try again")

// Todo is one card.
type Todo struct {
	ID       string     `json:"id"`   // loc:line for a checkbox item, loc for a task file
	Kind     string     `json:"kind"` // TodoCheckbox or TodoTask
	Status   TodoStatus `json:"status"`
	Title    string     `json:"title"`
	Labels   []string   `json:"labels"`
	Loc      string     `json:"loc"`
	Line     int        `json:"line"` // the item's line; for a task, its title line (or 1)
	Key      string     `json:"key"`  // graph node of the section holding the item, or the doc
	Doc      string     `json:"doc"`  // doc title
	Section  string     `json:"section,omitempty"`
	After    string     `json:"after,omitempty"` // a task's "After:" dependency
	Raw      string     `json:"raw"`             // the item's line as read (edits check it); empty for a task
	Subtasks []Subtask  `json:"subtasks"`
	Done     int        `json:"done"` // subtasks done
}

// Subtask is a checkbox item under a card.
type Subtask struct {
	Line  int    `json:"line"`
	Depth int    `json:"depth"` // 0 for a direct subtask, 1 for one under it, ...
	Title string `json:"title"`
	Done  bool   `json:"done"`
	Raw   string `json:"raw"`
}

// TaskStatus says whether loc is a task file of the queue, and its column.
func TaskStatus(loc string) (TodoStatus, bool) {
	dir, file := path.Split(loc)
	if !strings.EqualFold(path.Ext(file), ".md") {
		return "", false
	}
	dir = strings.TrimSuffix(dir, "/")
	if dir == TasksDir {
		return TodoReady, true
	}
	parent, sub := path.Split(dir)
	if strings.TrimSuffix(parent, "/") == TasksDir {
		if st, ok := folderStatus[sub]; ok && sub != "" {
			return st, true
		}
	}
	return "", false
}

// indentWidth counts leading whitespace, a tab as 4.
func indentWidth(s string) int {
	n := 0
	for _, c := range s {
		switch c {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

// todoText splits a checkbox item's text into its display title and labels.
func todoText(text string) (string, []string) {
	labels := []string{}
	for _, m := range labelRe.FindAllStringSubmatch(text, -1) {
		labels = appendLabel(labels, m[2])
	}
	title := labelRe.ReplaceAllString(text, "$1")
	title = linkRe.ReplaceAllString(title, "$1")
	title = emphasisRe.ReplaceAllString(title, "")
	return strings.Join(strings.Fields(title), " "), labels
}

func appendLabel(labels []string, l string) []string {
	l = NormalizeLabel(l)
	if l == "" {
		return labels
	}
	for _, have := range labels {
		if have == l {
			return labels
		}
	}
	return append(labels, l)
}

// NormalizeLabel makes a label one lowercase token: "#Launch Plan" → "launch-plan".
// It returns "" for a label with no letters or digits.
func NormalizeLabel(l string) string {
	l = strings.ToLower(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), "#")))
	l = strings.Join(strings.Fields(l), "-")
	if m := labelRe.FindStringSubmatch("#" + l); m == nil || m[2] != l {
		return ""
	}
	return l
}

// ParseTodos finds the cards of one doc.
func ParseTodos(doc *ParsedDoc) []Todo {
	lines := strings.Split(doc.Text, "\n")
	_, start := parseFrontmatter(lines)
	status, isTask := TaskStatus(doc.Loc)

	var task *Todo
	if isTask {
		task = &Todo{ID: doc.Loc, Kind: TodoTask, Status: status, Title: doc.Title, Labels: []string{}, Loc: doc.Loc, Line: 1,
			Key: doc.Loc, Doc: doc.Title, Subtasks: []Subtask{}}
	}
	sectionAt := func(line int) (string, string) {
		key, title := doc.Loc, ""
		for _, s := range doc.Sections {
			if s.Line <= line {
				key, title = s.Key, s.Title
			}
		}
		return key, title
	}

	var todos []Todo
	type open struct{ card, indent int }
	cur := open{card: -1}
	var subIndents []int // indents of the open card's (or task's) subtask levels
	depthOf := func(indent int) int {
		for len(subIndents) > 0 && subIndents[len(subIndents)-1] > indent {
			subIndents = subIndents[:len(subIndents)-1]
		}
		if len(subIndents) == 0 || subIndents[len(subIndents)-1] < indent {
			subIndents = append(subIndents, indent)
		}
		return len(subIndents) - 1
	}
	titleSeen, inFence := false, false
	for i := start; i < len(lines); i++ {
		line := lines[i]
		if fenceRe.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if isTask {
			trimmed := strings.TrimSpace(line)
			if m := headingRe.FindStringSubmatch(line); m != nil && len(m[1]) == 1 && !titleSeen {
				titleSeen, task.Line = true, i+1
			}
			if m := labelsLineRe.FindStringSubmatch(trimmed); m != nil {
				for _, l := range strings.Split(m[1], ",") {
					task.Labels = appendLabel(task.Labels, l)
				}
			}
			if m := afterLineRe.FindStringSubmatch(trimmed); m != nil {
				task.After = strings.TrimSpace(m[1])
			}
		}
		m := checkboxRe.FindStringSubmatch(line)
		if m == nil {
			// A heading, or text at or left of the card's own indent, closes the card;
			// plain sub-bullets and blank lines under it do not.
			if cur.card >= 0 && strings.TrimSpace(line) != "" && (headingRe.MatchString(line) || indentWidth(line) <= cur.indent) {
				cur.card = -1
			}
			continue
		}
		indent, marker := indentWidth(m[1]), m[3][0]
		title, _ := todoText(m[4])
		sub := Subtask{Line: i + 1, Title: title, Done: marker == 'x' || marker == 'X', Raw: line}
		switch {
		case isTask:
			sub.Depth = depthOf(indent)
			task.Subtasks = append(task.Subtasks, sub)
		case cur.card >= 0 && indent > cur.indent:
			sub.Depth = depthOf(indent)
			todos[cur.card].Subtasks = append(todos[cur.card].Subtasks, sub)
		default:
			key, section := sectionAt(i + 1)
			title, labels := todoText(m[4])
			todos = append(todos, Todo{ID: fmt.Sprintf("%s:%d", doc.Loc, i+1), Kind: TodoCheckbox, Status: markerStatus[marker],
				Title: title, Labels: labels, Loc: doc.Loc, Line: i + 1, Key: key, Doc: doc.Title, Section: section, Raw: line, Subtasks: []Subtask{}})
			cur, subIndents = open{card: len(todos) - 1, indent: indent}, nil
		}
	}
	if task != nil {
		todos = []Todo{*task}
	}
	for i := range todos {
		for _, s := range todos[i].Subtasks {
			if s.Done {
				todos[i].Done++
			}
		}
	}
	return todos
}

// Todos lists the cards of every tracked doc, in file order.
func (s *Store) Todos() ([]Todo, error) {
	locs, err := s.ScanDocs()
	if err != nil {
		return nil, err
	}
	todos := []Todo{}
	for _, loc := range locs {
		doc, err := s.ParseDoc(loc)
		if err != nil {
			continue // removed since the scan
		}
		todos = append(todos, ParseTodos(doc)...)
	}
	return todos, nil
}

// TodoQuery filters the board and pages each column, so the view can load a
// long column as it scrolls.
type TodoQuery struct {
	Status TodoStatus // one column; "" for all of them
	Kind   string     // TodoCheckbox or TodoTask; "" for both
	Labels []string   // a card needs every one
	Search string     // every term must appear in the title, a #label, the doc, section, path or a subtask
	Offset int
	Limit  int // cards per column; 0 for all
}

// TodoColumn is one page of a column.
type TodoColumn struct {
	Status TodoStatus `json:"status"`
	Total  int        `json:"total"` // the column's cards that pass the filters
	Todos  []Todo     `json:"todos"` // the page: Offset up to Offset+Limit
}

// TodoPage is the board for one query.
type TodoPage struct {
	Columns []TodoColumn `json:"columns"`
	Total   int          `json:"total"` // every card on the board, before the filters
}

// Match reports whether a card passes the query's filters (not its column or page).
func (q TodoQuery) Match(t Todo) bool {
	if q.Kind != "" && t.Kind != q.Kind {
		return false
	}
	for _, l := range q.Labels {
		if l = NormalizeLabel(l); l != "" && !slices.Contains(t.Labels, l) {
			return false
		}
	}
	terms := strings.Fields(strings.ToLower(q.Search))
	if len(terms) == 0 {
		return true
	}
	parts := []string{t.Title, t.Doc, t.Section, t.Loc}
	for _, l := range t.Labels {
		parts = append(parts, "#"+l)
	}
	for _, s := range t.Subtasks {
		parts = append(parts, s.Title)
	}
	text := strings.ToLower(strings.Join(parts, " "))
	for _, term := range terms {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}

// TodoPage reads the board, filters it, and returns one page of each column
// (or of q.Status alone), in file order.
func (s *Store) TodoPage(q TodoQuery) (TodoPage, error) {
	if q.Status != "" && !slices.Contains(TodoStatuses, q.Status) {
		return TodoPage{}, fmt.Errorf("unknown column %q", q.Status)
	}
	if q.Kind != "" && q.Kind != TodoCheckbox && q.Kind != TodoTask {
		return TodoPage{}, fmt.Errorf("unknown card kind %q", q.Kind)
	}
	q.Offset = max(q.Offset, 0)
	todos, err := s.Todos()
	if err != nil {
		return TodoPage{}, err
	}
	page := TodoPage{Columns: []TodoColumn{}, Total: len(todos)}
	for _, st := range TodoStatuses {
		if q.Status != "" && st != q.Status {
			continue
		}
		col := TodoColumn{Status: st, Todos: []Todo{}}
		for _, t := range todos {
			if t.Status != st || !q.Match(t) {
				continue
			}
			if col.Total >= q.Offset && (q.Limit <= 0 || col.Total < q.Offset+q.Limit) {
				col.Todos = append(col.Todos, t)
			}
			col.Total++
		}
		page.Columns = append(page.Columns, col)
	}
	return page, nil
}

// TodoEdit is one change the board makes to a card, in its markdown file.
type TodoEdit struct {
	Op     string     `json:"op"`   // "move", "labels", "subtask" (add one) or "check" (tick a subtask)
	Kind   string     `json:"kind"` // TodoCheckbox or TodoTask
	Loc    string     `json:"loc"`
	Line   int        `json:"line"`   // the checkbox item; for "check", the subtask
	Raw    string     `json:"raw"`    // that line as the board read it
	To     TodoStatus `json:"to"`     // move
	Labels []string   `json:"labels"` // labels: the card's new list
	Text   string     `json:"text"`   // subtask
	Done   bool       `json:"done"`   // check
}

// EditTodo writes e into the card's file and returns the file's loc afterwards
// (a moved task has a new one). It edits only tracked docs, and only the line(s)
// the change needs; the caller syncs the graph afterwards.
func (s *Store) EditTodo(e TodoEdit) (string, error) {
	locs, err := s.ScanDocs()
	if err != nil {
		return "", err
	}
	tracked := false
	for _, l := range locs {
		tracked = tracked || l == e.Loc
	}
	if !tracked {
		return "", fmt.Errorf("%s is not a tracked doc", e.Loc)
	}
	if e.Kind == TodoTask {
		if _, ok := TaskStatus(e.Loc); !ok {
			return "", fmt.Errorf("%s is not a task file in %s/", e.Loc, TasksDir)
		}
	}
	switch e.Op {
	case "move":
		if _, ok := statusMarker[e.To]; !ok {
			return "", fmt.Errorf("unknown column %q", e.To)
		}
		if e.Kind == TodoTask {
			return s.moveTask(e.Loc, e.To)
		}
		return e.Loc, s.rewriteDoc(e.Loc, func(lines []string) ([]string, error) {
			return setMarker(lines, e.Line, e.Raw, statusMarker[e.To])
		})
	case "check":
		marker := byte(' ')
		if e.Done {
			marker = 'x'
		}
		return e.Loc, s.rewriteDoc(e.Loc, func(lines []string) ([]string, error) { return setMarker(lines, e.Line, e.Raw, marker) })
	case "labels":
		labels := []string{}
		for _, l := range e.Labels {
			labels = appendLabel(labels, l)
		}
		if e.Kind == TodoTask {
			return e.Loc, s.rewriteDoc(e.Loc, func(lines []string) ([]string, error) { return setTaskLabels(lines, labels), nil })
		}
		return e.Loc, s.rewriteDoc(e.Loc, func(lines []string) ([]string, error) { return setItemLabels(lines, e.Line, e.Raw, labels) })
	case "subtask":
		text := strings.Join(strings.Fields(e.Text), " ")
		if text == "" {
			return "", errors.New("a subtask needs text")
		}
		if e.Kind == TodoTask {
			return e.Loc, s.rewriteDoc(e.Loc, func(lines []string) ([]string, error) { return addTaskSubtask(lines, text), nil })
		}
		return e.Loc, s.rewriteDoc(e.Loc, func(lines []string) ([]string, error) { return addItemSubtask(lines, e.Line, e.Raw, text) })
	}
	return "", fmt.Errorf("unknown edit %q", e.Op)
}

// moveTask moves a task file to the folder of column to.
func (s *Store) moveTask(loc string, to TodoStatus) (string, error) {
	dest := path.Join(TasksDir, statusFolder[to], path.Base(loc))
	if dest == loc {
		return loc, nil
	}
	from := filepath.Join(s.Root, filepath.FromSlash(loc))
	if _, err := os.Stat(from); err != nil {
		return "", ErrTodoChanged
	}
	target := filepath.Join(s.Root, filepath.FromSlash(dest))
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("%s already exists", dest)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	return dest, os.Rename(from, target)
}

// rewriteDoc applies fn to the doc's lines and writes the result back atomically,
// keeping CRLF line endings when the file has them.
func (s *Store) rewriteDoc(loc string, fn func([]string) ([]string, error)) error {
	file := filepath.Join(s.Root, filepath.FromSlash(loc))
	info, err := os.Stat(file)
	if err != nil {
		return ErrTodoChanged
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return ErrTodoChanged
	}
	crlf := strings.Contains(string(data), "\r\n")
	lines, err := fn(strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"))
	if err != nil {
		return err
	}
	out := strings.Join(lines, "\n")
	if crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".board-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.WriteString(out); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil { // CreateTemp makes it 0600
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), file); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// itemAt returns the checkbox match of line n (1-based) if it is still raw.
func itemAt(lines []string, n int, raw string) ([]int, error) {
	if n < 1 || n > len(lines) || lines[n-1] != strings.TrimRight(raw, "\r") {
		return nil, ErrTodoChanged
	}
	m := checkboxRe.FindStringSubmatchIndex(lines[n-1])
	if m == nil {
		return nil, ErrTodoChanged
	}
	return m, nil
}

func setMarker(lines []string, n int, raw string, marker byte) ([]string, error) {
	m, err := itemAt(lines, n, raw)
	if err != nil {
		return nil, err
	}
	l := lines[n-1]
	lines[n-1] = l[:m[6]] + string(marker) + l[m[7]:]
	return lines, nil
}

func setItemLabels(lines []string, n int, raw string, labels []string) ([]string, error) {
	m, err := itemAt(lines, n, raw)
	if err != nil {
		return nil, err
	}
	l := lines[n-1]
	prefix, text := l[:m[7]+1]+" ", "" // "  - [x] "
	if m[8] >= 0 {
		text = l[m[8]:m[9]]
	}
	text = strings.TrimSpace(labelRe.ReplaceAllString(text, "$1"))
	text = strings.Join(strings.Fields(text), " ")
	for _, lb := range labels {
		text += " #" + lb
	}
	lines[n-1] = prefix + strings.TrimSpace(text)
	return lines, nil
}

// setTaskLabels rewrites the task's "Labels:" line, adds one under its title, or
// removes it when labels is empty.
func setTaskLabels(lines []string, labels []string) []string {
	want := "Labels: " + strings.Join(labels, ", ")
	title, inFence := -1, false
	for i, l := range lines {
		if fenceRe.MatchString(l) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if labelsLineRe.MatchString(strings.TrimSpace(l)) {
			if len(labels) == 0 {
				return append(lines[:i], lines[i+1:]...)
			}
			lines[i] = want
			return lines
		}
		if m := headingRe.FindStringSubmatch(l); m != nil && len(m[1]) == 1 && title < 0 {
			title = i
		}
	}
	if len(labels) == 0 {
		return lines
	}
	at := title + 1 // the top of the file when there is no title
	insert := []string{"", want}
	if title < 0 {
		insert = []string{want, ""}
	}
	return append(lines[:at], append(insert, lines[at:]...)...)
}

// addItemSubtask adds "- [ ] text" as the last subtask of the checkbox item at line n.
func addItemSubtask(lines []string, n int, raw, text string) ([]string, error) {
	m, err := itemAt(lines, n, raw)
	if err != nil {
		return nil, err
	}
	parent := lines[n-1]
	indent := indentWidth(parent[m[2]:m[3]])
	child := strings.Repeat(" ", indent+len(parent[m[4]:m[5]])+1) + "- [ ] " + text
	last, styled := n-1, false // index of the block's last line; whether child copies a subtask
	for i := n; i < len(lines); i++ {
		l := lines[i]
		if strings.TrimSpace(l) == "" {
			continue
		}
		if headingRe.MatchString(l) || indentWidth(l) <= indent {
			break
		}
		if c := checkboxRe.FindStringSubmatch(l); c != nil && !styled {
			child, styled = c[1]+c[2]+" [ ] "+text, true // follow the existing subtasks' indent and bullet
		}
		last = i
	}
	return append(lines[:last+1], append([]string{child}, lines[last+1:]...)...), nil
}

// addTaskSubtask adds "- [ ] text" after the task file's last checkbox item, or
// in a new "## Subtasks" section at its end.
func addTaskSubtask(lines []string, text string) []string {
	last, prefix, inFence := -1, "- ", false
	for i, l := range lines {
		if fenceRe.MatchString(l) {
			inFence = !inFence
			continue
		}
		if c := checkboxRe.FindStringSubmatch(l); c != nil && !inFence {
			if last < 0 {
				prefix = c[1] + c[2] + " "
			}
			last = i
		}
	}
	item := prefix + "[ ] " + text
	if last >= 0 {
		return append(lines[:last+1], append([]string{item}, lines[last+1:]...)...)
	}
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return append(lines[:end], "", "## Subtasks", "", item, "")
}
