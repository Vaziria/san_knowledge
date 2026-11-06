package engine

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// The JSON Lines and CSV steps (the user, 2026-11-06: "is we have jsonl file
// append node ?", then "yes, perline per item", "add node to read too" and
// "add node append csv and read csv too"): a workflow keeps records in a
// file, such as Google Maps Places' places, one line or row an item, and
// reads them back. Paths are placed as File Path places a name.

// maxData is the largest file Read JSONL and Read CSV read.
const maxData = 50 << 20

// separators are the CSV steps' Separator options: Semicolon is what Excel
// writes and reads when Windows is set to Indonesian.
var separators = map[string]rune{"Comma": ',', "Semicolon": ';', "Tab": '\t'}

var dataFileKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "append-jsonl", Title: "Append JSONL", Category: "Storage",
			Description: "Appends Record to the JSON Lines file at Path, one JSON value a line, in a storage when one is picked, else in the tower's data folder or an absolute path: a list adds one line per item (Google Maps Places' Places: a line a place), anything else one line. JSON, typed text such as {\"name\": \"Kopi\"} or a list [...], is used when nothing is linked to Record. A missing file and its folders are made. A call's lines are written at once, so two runs appending to one file do not mix. File is the file's absolute path, Lines how many were added.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("File", Text), sock("Lines", Text)},
			Settings:    []*v1.Setting{{Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"}},
			Inputs: []*v1.Socket{sock("Event", Event), hint("leads/places.jsonl", field("Path")), sock("Record", Object),
				hint(`{"name": "Kopi"}, when Record is not linked`, field("JSON"))},
		},
		run: appendJSONL,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "read-jsonl", Title: "Read JSONL", Category: "Storage",
			Description: "Reads the JSON Lines file at Path (up to 50 MB), in a storage when one is picked, else in the tower's data folder or an absolute path: Records is the list of its values, one a line; Each fires once per record with Item it, then Done; Count is how many. Blank lines are skipped, and so is a line that is not JSON (such as one cut off by a crash), which the log names.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Each", Event), sock("Item", Object), sock("Records", Object), sock("Count", Text)},
			Settings:    []*v1.Setting{{Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"}},
			Inputs:      []*v1.Socket{sock("Event", Event), hint("leads/places.jsonl", field("Path"))},
		},
		run: readJSONL,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "append-csv", Title: "Append CSV", Category: "Storage",
			Description: "Appends Record to the CSV file at Path as rows, in a storage when one is picked, else in the tower's data folder or an absolute path: a list adds one row per item, anything else one row. An object's values go under the columns of the file's header row; a new file gets its header from Columns (names split by commas), else from the first record's keys, sorted. A key the header lacks is left out, and logged; a list or object value is written as JSON, a missing one as empty. An item that is itself a list is a row of its values, in order. JSON, typed text, is used when nothing is linked to Record. Separator: Comma, Semicolon (what Excel uses when Windows is set to Indonesian) or Tab. A missing file and its folders are made, and a call's rows are written at once. File is the file's absolute path, Rows how many were added, Columns the header.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("File", Text), sock("Rows", Text), sock("Columns", Text)},
			Settings: []*v1.Setting{{Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"},
				{Name: "Separator", Options: []string{"Comma", "Semicolon", "Tab"}}},
			Inputs: []*v1.Socket{sock("Event", Event), hint("leads/places.csv", field("Path")), sock("Record", Object),
				hint(`{"name": "Kopi"}, when Record is not linked`, field("JSON")),
				hint("name, phone, address: a new file's header", field("Columns"))},
		},
		run: appendCSV,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "read-csv", Title: "Read CSV", Category: "Storage",
			Description: "Reads the CSV file at Path (up to 50 MB), in a storage when one is picked, else in the tower's data folder or an absolute path. Header First Row makes each later row an object of its cells (text) by the first row's names; None makes each row a list. Separator Auto takes whichever of comma, semicolon and tab the first line has most of. Records is the list of rows; Each fires once per row with Item it, then Done; Count is how many, Columns the header's names.",
			Outputs: []*v1.Socket{sock("Done", Event), sock("Each", Event), sock("Item", Object), sock("Records", Object), sock("Count", Text),
				sock("Columns", Object)},
			Settings: []*v1.Setting{{Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"},
				{Name: "Separator", Options: []string{"Auto", "Comma", "Semicolon", "Tab"}},
				{Name: "Header", Options: []string{"First Row", "None"}}},
			Inputs: []*v1.Socket{sock("Event", Event), hint("leads/places.csv", field("Path"))},
		},
		run: readCSV,
	},
}

// fileLocks has a mutex per file the append steps write, so the runs of a
// tower append to one file one at a time.
var fileLocks sync.Map

func lockFile(full string) func() {
	key := full
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	m, _ := fileLocks.LoadOrStore(key, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// dataPath reads Path and places it; the file must not be a folder.
func dataPath(c *call) (string, error) {
	raw := strings.TrimSpace(c.text("Path"))
	if raw == "" {
		return "", fmt.Errorf("Path %w", errEmpty)
	}
	full, err := place(c, "Path", raw)
	if err != nil {
		return "", err
	}
	if fi, err := os.Stat(filepath.FromSlash(full)); err == nil && fi.IsDir() {
		return "", fmt.Errorf("Path %s is a folder: give a file", full)
	}
	return full, nil
}

// records is what an append step writes: Record when something is linked,
// else the JSON typed; a list is its items, anything else one record.
func records(c *call) ([]*structpb.Value, error) {
	v := c.object("Record")
	if _, null := v.GetKind().(*structpb.Value_NullValue); null || v.GetKind() == nil {
		text := strings.TrimSpace(c.text("JSON"))
		if text == "" {
			return nil, errors.New("Record: link a value to write, or type JSON")
		}
		v = &structpb.Value{}
		if err := protojson.Unmarshal([]byte(text), v); err != nil {
			return nil, fmt.Errorf("JSON is not JSON: %v", err)
		}
	}
	if l, ok := v.GetKind().(*structpb.Value_ListValue); ok {
		return l.ListValue.GetValues(), nil
	}
	return []*structpb.Value{v}, nil
}

// appendTo writes b at the end of the file at full, making it and its
// folders when missing. A file that does not end with a line end gets one
// first, so the new lines are lines of their own.
func appendTo(full string, b []byte) error {
	p := filepath.FromSlash(full)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if fi, err := f.Stat(); err == nil && fi.Size() > 0 {
		last := make([]byte, 1)
		if _, err := f.ReadAt(last, fi.Size()-1); err == nil && last[0] != '\n' {
			b = append([]byte("\n"), b...)
		}
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func appendJSONL(c *call) error {
	full, err := dataPath(c)
	if err != nil {
		return err
	}
	list, err := records(c)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	for _, v := range list {
		line, err := stringify(v, false)
		if err != nil {
			return err
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if len(list) > 0 {
		defer lockFile(full)()
		if err := appendTo(full, b.Bytes()); err != nil {
			return err
		}
	}
	c.logs = append(c.logs, fmt.Sprintf("appended %d lines to %s", len(list), full))
	c.outputs["File"] = stringValue(full)
	c.outputs["Lines"] = stringValue(strconv.Itoa(len(list)))
	c.fire("Done")
	return nil
}

// readData reads a data file whole, without its byte order mark.
func readData(full string) ([]byte, error) {
	fi, err := os.Stat(filepath.FromSlash(full))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("Path %s is not there", full)
	case err != nil:
		return nil, err
	case fi.Size() > maxData:
		return nil, fmt.Errorf("Path %s is %d MB: the read steps read files up to %d MB", full, fi.Size()>>20, maxData>>20)
	}
	b, err := os.ReadFile(filepath.FromSlash(full))
	return bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), err
}

func readJSONL(c *call) error {
	full, err := dataPath(c)
	if err != nil {
		return err
	}
	b, err := readData(full)
	if err != nil {
		return err
	}
	list := &structpb.ListValue{}
	var bad []string
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 64<<10), maxData)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		v := &structpb.Value{}
		if !utf8.Valid(line) || protojson.Unmarshal(line, v) != nil {
			bad = append(bad, strconv.Itoa(n))
			continue
		}
		list.Values = append(list.Values, v)
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("Path %s: %v", full, err)
	}
	if len(bad) > 0 {
		c.logs = append(c.logs, fmt.Sprintf("skipped %d lines that are not JSON: line %s", len(bad), strings.Join(firstOf(bad, 10), ", ")))
	}
	return eachRecord(c, list)
}

// eachRecord gives a read step's Records and Count, then fires Each once per
// record with Item it, then Done.
func eachRecord(c *call, list *structpb.ListValue) error {
	c.outputs["Records"] = structpb.NewListValue(list)
	c.outputs["Count"] = stringValue(strconv.Itoa(len(list.Values)))
	c.outputs["Item"] = structpb.NewNullValue()
	for _, v := range list.Values {
		if c.each == nil {
			break
		}
		c.outputs["Item"] = v
		if err := c.each("Each"); err != nil {
			return err
		}
	}
	c.fire("Done")
	return nil
}

func firstOf(s []string, n int) []string {
	if len(s) > n {
		return append(slices.Clone(s[:n]), "...")
	}
	return s
}

// cell is a value as a CSV cell: text as it is, numbers without a needless
// decimal point, a list or object as JSON, null as empty.
func cell(v *structpb.Value) (string, error) {
	switch k := v.GetKind().(type) {
	case nil, *structpb.Value_NullValue:
		return "", nil
	case *structpb.Value_StringValue:
		return k.StringValue, nil
	case *structpb.Value_BoolValue:
		return strconv.FormatBool(k.BoolValue), nil
	case *structpb.Value_NumberValue:
		return strconv.FormatFloat(k.NumberValue, 'f', -1, 64), nil
	default:
		return stringify(v, false)
	}
}

// splitNames reads a list of names split by commas.
func splitNames(s string) []string {
	var out []string
	for _, n := range strings.Split(s, ",") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// headerOf reads the first row of the CSV file at full: nil when the file
// is missing or empty.
func headerOf(full string, sep rune) ([]string, error) {
	f, err := os.Open(filepath.FromSlash(full))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	if bom, _ := br.Peek(3); bytes.Equal(bom, []byte("\xef\xbb\xbf")) {
		br.Discard(3)
	}
	r := csv.NewReader(br)
	r.Comma, r.FieldsPerRecord, r.LazyQuotes = sep, -1, true
	row, err := r.Read()
	if err == io.EOF {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Path %s: its header row is not CSV: %v", full, err)
	}
	return row, nil
}

func appendCSV(c *call) error {
	sep, ok := separators[c.setting("Separator")]
	if c.setting("Separator") == "" {
		sep, ok = ',', true
	}
	if !ok {
		return fmt.Errorf("Separator %q: give Comma, Semicolon or Tab", c.setting("Separator"))
	}
	full, err := dataPath(c)
	if err != nil {
		return err
	}
	list, err := records(c)
	if err != nil {
		return err
	}
	defer lockFile(full)()
	header, err := headerOf(full, sep)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.Comma = sep
	if header == nil {
		header = splitNames(c.text("Columns"))
		if len(header) == 0 {
			for _, v := range list {
				if s, ok := v.GetKind().(*structpb.Value_StructValue); ok {
					for k := range s.StructValue.GetFields() {
						header = append(header, k)
					}
					slices.Sort(header)
					break
				}
			}
		}
		if len(header) == 0 {
			if len(list) == 0 {
				return errors.New("Columns: a new file needs a header; give Columns, or records that are objects")
			}
		} else if err := w.Write(header); err != nil {
			return err
		}
	}
	known := map[string]bool{}
	for _, h := range header {
		known[h] = true
	}
	left := map[string]bool{}
	for _, v := range list {
		var row []string
		switch k := v.GetKind().(type) {
		case *structpb.Value_StructValue:
			fields := k.StructValue.GetFields()
			for _, h := range header {
				s, err := cell(fields[h])
				if err != nil {
					return err
				}
				row = append(row, s)
			}
			for name := range fields {
				if !known[name] {
					left[name] = true
				}
			}
		case *structpb.Value_ListValue:
			for _, x := range k.ListValue.GetValues() {
				s, err := cell(x)
				if err != nil {
					return err
				}
				row = append(row, s)
			}
		default:
			s, err := cell(v)
			if err != nil {
				return err
			}
			row = []string{s}
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	if b.Len() > 0 {
		if err := appendTo(full, b.Bytes()); err != nil {
			return err
		}
	}
	if len(left) > 0 {
		names := make([]string, 0, len(left))
		for n := range left {
			names = append(names, n)
		}
		slices.Sort(names)
		c.logs = append(c.logs, "left out, not in the header: "+strings.Join(names, ", "))
	}
	c.logs = append(c.logs, fmt.Sprintf("appended %d rows to %s", len(list), full))
	c.outputs["File"] = stringValue(full)
	c.outputs["Rows"] = stringValue(strconv.Itoa(len(list)))
	c.outputs["Columns"] = stringValue(strings.Join(header, ", "))
	c.fire("Done")
	return nil
}

// sniff is the separator a CSV file's first line has most of: comma,
// semicolon or tab.
func sniff(b []byte) rune {
	line, _, _ := bytes.Cut(b, []byte("\n"))
	best, most := ',', 0
	for _, r := range []rune{',', ';', '\t'} {
		if n := bytes.Count(line, []byte(string(r))); n > most {
			best, most = r, n
		}
	}
	return best
}

func readCSV(c *call) error {
	full, err := dataPath(c)
	if err != nil {
		return err
	}
	b, err := readData(full)
	if err != nil {
		return err
	}
	var sep rune
	switch s := c.setting("Separator"); s {
	case "", "Auto":
		sep = sniff(b)
	default:
		var ok bool
		if sep, ok = separators[s]; !ok {
			return fmt.Errorf("Separator %q: give Auto, Comma, Semicolon or Tab", s)
		}
	}
	if !utf8.Valid(b) {
		return fmt.Errorf("Path %s is not UTF-8 text", full)
	}
	r := csv.NewReader(bytes.NewReader(b))
	r.Comma, r.FieldsPerRecord, r.LazyQuotes = sep, -1, true
	rows, err := r.ReadAll()
	if err != nil {
		return fmt.Errorf("Path %s is not CSV: %v", full, err)
	}
	list := &structpb.ListValue{}
	columns := &structpb.ListValue{}
	switch c.setting("Header") {
	case "", "First Row":
		var names []string
		if len(rows) > 0 {
			names = columnNames(rows[0])
			rows = rows[1:]
		}
		for _, n := range names {
			columns.Values = append(columns.Values, structpb.NewStringValue(n))
		}
		for _, row := range rows {
			fields := map[string]*structpb.Value{}
			for i, s := range row {
				name := fmt.Sprintf("column %d", i+1)
				if i < len(names) {
					name = names[i]
				}
				fields[name] = structpb.NewStringValue(s)
			}
			for i := len(row); i < len(names); i++ {
				fields[names[i]] = structpb.NewStringValue("")
			}
			list.Values = append(list.Values, structpb.NewStructValue(&structpb.Struct{Fields: fields}))
		}
	case "None":
		for _, row := range rows {
			cells := &structpb.ListValue{}
			for _, s := range row {
				cells.Values = append(cells.Values, structpb.NewStringValue(s))
			}
			list.Values = append(list.Values, structpb.NewListValue(cells))
		}
	default:
		return fmt.Errorf("Header %q: give First Row or None", c.setting("Header"))
	}
	c.outputs["Columns"] = structpb.NewListValue(columns)
	return eachRecord(c, list)
}

// columnNames are a header row's names: an empty one is "column N", and a
// name seen before gets "_2", "_3".
func columnNames(header []string) []string {
	seen := map[string]int{}
	out := make([]string, len(header))
	for i, h := range header {
		h = strings.TrimSpace(h)
		if h == "" {
			h = fmt.Sprintf("column %d", i+1)
		}
		if seen[h]++; seen[h] > 1 {
			h = fmt.Sprintf("%s_%d", h, seen[h])
		}
		out[i] = h
	}
	return out
}
