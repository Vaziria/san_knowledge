package engine

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// fileCall is a call of a data file step in the data folder data, with its
// settings and text inputs as name, value pairs, and Record when given.
func fileCall(data, kind string, record *structpb.Value, pairs ...string) *call {
	st := step("s", kind)
	in := map[string]proto.Message{}
	for i := 0; i+1 < len(pairs); i += 2 {
		switch pairs[i] {
		case "Storage", "Separator", "Header":
			st.Values[pairs[i]] = pairs[i+1]
		default:
			in[pairs[i]] = wrapperspb.String(pairs[i+1])
		}
	}
	if record != nil {
		in["Record"] = record
	}
	return &call{ctx: context.Background(), env: Env{DataDir: data}, step: st, inputs: in, outputs: map[string]proto.Message{}}
}

func mustValue(t *testing.T, v any) *structpb.Value {
	t.Helper()
	sv, err := structpb.NewValue(v)
	if err != nil {
		t.Fatal(err)
	}
	return sv
}

func outText(c *call, name string) string {
	if v, ok := c.outputs[name].(*wrapperspb.StringValue); ok {
		return v.GetValue()
	}
	return ""
}

func readText(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(p))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAppendJSONL(t *testing.T) {
	data := filepath.ToSlash(t.TempDir())
	places := mustValue(t, []any{
		map[string]any{"name": "Toko Kopi Style", "phone": "0813-2345-6387", "latitude": -6.1931299, "social": []any{map[string]any{"site": "instagram"}}},
		map[string]any{"name": "Shan Coffee", "reviews": 103.0},
	})
	c := fileCall(data, "append-jsonl", places, "Path", "leads/places.jsonl")
	if err := appendJSONL(c); err != nil {
		t.Fatal(err)
	}
	file := data + "/leads/places.jsonl"
	if outText(c, "File") != file || outText(c, "Lines") != "2" || strings.Join(c.fired, ",") != "Done" {
		t.Errorf("outputs %v, fired %v", c.outputs, c.fired)
	}
	want := `{"latitude":-6.1931299,"name":"Toko Kopi Style","phone":"0813-2345-6387","social":[{"site":"instagram"}]}` + "\n" +
		`{"name":"Shan Coffee","reviews":103}` + "\n"
	if got := readText(t, file); got != want {
		t.Fatalf("file =\n%s\nwant\n%s", got, want)
	}

	// One object is one line; typed JSON when Record is not linked; a file
	// left without its last line end gets one first.
	if err := os.WriteFile(filepath.FromSlash(file), []byte(strings.TrimSuffix(want, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendJSONL(fileCall(data, "append-jsonl", mustValue(t, map[string]any{"name": "Kopi < & >"}), "Path", "leads/places.jsonl")); err != nil {
		t.Fatal(err)
	}
	if err := appendJSONL(fileCall(data, "append-jsonl", nil, "Path", "leads/places.jsonl", "JSON", ` [{"n": 1}, "two", 3] `)); err != nil {
		t.Fatal(err)
	}
	if got, w := readText(t, file), want+`{"name":"Kopi < & >"}`+"\n"+`{"n":1}`+"\n\"two\"\n3\n"; got != w {
		t.Fatalf("file =\n%s\nwant\n%s", got, w)
	}

	for name, c := range map[string]*call{
		"no path":     fileCall(data, "append-jsonl", places),
		"no record":   fileCall(data, "append-jsonl", nil, "Path", "x.jsonl"),
		"bad json":    fileCall(data, "append-jsonl", nil, "Path", "x.jsonl", "JSON", "{name}"),
		"a folder":    fileCall(data, "append-jsonl", places, "Path", "leads"),
		"out of data": fileCall(data, "append-jsonl", places, "Path", "../x.jsonl", "Storage", "1"),
	} {
		if err := appendJSONL(c); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// Runs appending to one file at once do not mix their lines.
func TestAppendJSONLTogether(t *testing.T) {
	data := filepath.ToSlash(t.TempDir())
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var items []any
			for j := range 50 {
				items = append(items, map[string]any{"run": float64(i), "n": float64(j), "pad": strings.Repeat("x", 200)})
			}
			sv, _ := structpb.NewValue(items)
			if err := appendJSONL(fileCall(data, "append-jsonl", sv, "Path", "all.jsonl")); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	c := fileCall(data, "read-jsonl", nil, "Path", "all.jsonl")
	if err := readJSONL(c); err != nil {
		t.Fatal(err)
	}
	if outText(c, "Count") != "1000" || len(c.logs) != 0 {
		t.Fatalf("Count %s, logs %v", outText(c, "Count"), c.logs)
	}
	// Each run's lines are together and in order.
	last := map[float64]float64{}
	for _, v := range c.outputs["Records"].(*structpb.Value).GetListValue().GetValues() {
		f := v.GetStructValue().GetFields()
		run, n := f["run"].GetNumberValue(), f["n"].GetNumberValue()
		if prev, ok := last[run]; (ok && n != prev+1) || (!ok && n != 0) {
			t.Fatalf("run %v: line %v after %v", run, n, prev)
		}
		last[run] = n
	}
}

func TestReadJSONL(t *testing.T) {
	data := filepath.ToSlash(t.TempDir())
	body := "\xef\xbb\xbf{\"name\":\"a\"}\r\n\n  {\"name\":\"b\",\"n\":2}\nnot json\n[1,2]\n{\"name\":\"cut off"
	if err := os.WriteFile(filepath.Join(filepath.FromSlash(data), "x.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c := fileCall(data, "read-jsonl", nil, "Path", "x.jsonl")
	if err := readJSONL(c); err != nil {
		t.Fatal(err)
	}
	records := c.outputs["Records"].(*structpb.Value).GetListValue().GetValues()
	if outText(c, "Count") != "3" || len(records) != 3 || records[1].GetStructValue().GetFields()["n"].GetNumberValue() != 2 ||
		len(records[2].GetListValue().GetValues()) != 2 || strings.Join(c.fired, ",") != "Done" {
		t.Errorf("Count %s, records %v, fired %v", outText(c, "Count"), records, c.fired)
	}
	if len(c.logs) != 1 || c.logs[0] != "skipped 2 lines that are not JSON: line 4, 6" {
		t.Errorf("logs %q", c.logs)
	}
	if err := readJSONL(fileCall(data, "read-jsonl", nil, "Path", "gone.jsonl")); err == nil || !strings.Contains(err.Error(), "is not there") {
		t.Errorf("missing file: %v", err)
	}
}

// Read JSONL's Each runs its branch once per record, with Item that record.
func TestReadJSONLLoop(t *testing.T) {
	data := filepath.ToSlash(t.TempDir())
	if err := os.WriteFile(filepath.Join(filepath.FromSlash(data), "x.jsonl"), []byte("{\"name\":\"a\"}\n{\"name\":\"b\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("ev", TopicEvent, "Topic", "x"),
			step("r", "read-jsonl", "Path", "x.jsonl"),
			step("s", "stringify"),
			step("each", "log"),
			step("done", "log", "Message", "done"),
		},
		Links: []*v1.Link{
			link("ev", "Event", "r", "Event"),
			link("r", "Each", "each", "Event"),
			link("r", "Item", "s", "Object"),
			link("s", "Text", "each", "Message"),
			link("r", "Done", "done", "Event"),
		},
	}
	steps, err := Execute(context.Background(), g, "ev", nil, Env{DataDir: data})
	if err != nil {
		t.Fatal(err)
	}
	if got := ran(steps); !strings.Contains(got, `each::{"name":"a"}`) || !strings.Contains(got, `each::{"name":"b"}`) || !strings.HasSuffix(got, "done::done") {
		t.Errorf("ran %s", got)
	}
}

func TestAppendCSV(t *testing.T) {
	data := filepath.ToSlash(t.TempDir())
	places := mustValue(t, []any{
		map[string]any{"name": "Toko Kopi, Style", "phone": "0813", "rating": 4.9, "social": []any{"https://www.instagram.com/kopi.style"}},
		map[string]any{"name": "Shan \"Coffee\"", "reviews": 103.0},
	})
	c := fileCall(data, "append-csv", places, "Path", "leads/places.csv")
	if err := appendCSV(c); err != nil {
		t.Fatal(err)
	}
	file := data + "/leads/places.csv"
	want := "name,phone,rating,social\n" +
		"\"Toko Kopi, Style\",0813,4.9,\"[\"\"https://www.instagram.com/kopi.style\"\"]\"\n" +
		"\"Shan \"\"Coffee\"\"\",,,\n"
	if got := readText(t, file); got != want {
		t.Fatalf("file =\n%s\nwant\n%s", got, want)
	}
	if outText(c, "Rows") != "2" || outText(c, "Columns") != "name, phone, rating, social" || outText(c, "File") != file {
		t.Errorf("outputs %v", c.outputs)
	}
	if len(c.logs) != 2 || c.logs[0] != "left out, not in the header: reviews" {
		t.Errorf("logs %q", c.logs)
	}

	// The file's header wins over Columns; a row as a list goes as it is.
	c = fileCall(data, "append-csv", mustValue(t, []any{map[string]any{"phone": "0888", "name": "Kopi"}, []any{"a", 1.0, true}}),
		"Path", "leads/places.csv", "Columns", "phone")
	if err := appendCSV(c); err != nil {
		t.Fatal(err)
	}
	if got := readText(t, file); got != want+"Kopi,0888,,\na,1,true\n" {
		t.Fatalf("second append =\n%s", got)
	}

	// Columns names a new file's header, in its order; Semicolon.
	c = fileCall(data, "append-csv", nil, "Path", "semi.csv", "Separator", "Semicolon", "Columns", "phone, name", "JSON", `{"name": "Kopi; Susu", "phone": "0813"}`)
	if err := appendCSV(c); err != nil {
		t.Fatal(err)
	}
	if got := readText(t, data+"/semi.csv"); got != "phone;name\n0813;\"Kopi; Susu\"\n" {
		t.Fatalf("semicolon file =\n%s", got)
	}
	if err := appendCSV(fileCall(data, "append-csv", places, "Path", "x.csv", "Separator", "Pipe")); err == nil {
		t.Error("Separator Pipe: no error")
	}
}

func TestReadCSV(t *testing.T) {
	data := filepath.ToSlash(t.TempDir())
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(filepath.FromSlash(data), name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("excel.csv", "\xef\xbb\xbfname;phone;;name\r\n\"Kopi; Susu\";0813;x;dup\r\nShan;;\r\nLong;1;2;3;4\r\n")
	c := fileCall(data, "read-csv", nil, "Path", "excel.csv")
	if err := readCSV(c); err != nil {
		t.Fatal(err)
	}
	records := c.outputs["Records"].(*structpb.Value).GetListValue().GetValues()
	var cols []string
	for _, v := range c.outputs["Columns"].(*structpb.Value).GetListValue().GetValues() {
		cols = append(cols, v.GetStringValue())
	}
	if outText(c, "Count") != "3" || strings.Join(cols, "|") != "name|phone|column 3|name_2" {
		t.Fatalf("Count %s, Columns %v", outText(c, "Count"), cols)
	}
	get := func(i int, k string) string { return records[i].GetStructValue().GetFields()[k].GetStringValue() }
	if get(0, "name") != "Kopi; Susu" || get(0, "name_2") != "dup" || get(1, "phone") != "" || get(1, "name_2") != "" || get(2, "column 5") != "4" {
		t.Errorf("records %v", records)
	}

	// Header None: each row a list; a comma file read as such.
	write("plain.csv", "a,b\n1,\"2,3\"\n")
	c = fileCall(data, "read-csv", nil, "Path", "plain.csv", "Header", "None")
	if err := readCSV(c); err != nil {
		t.Fatal(err)
	}
	records = c.outputs["Records"].(*structpb.Value).GetListValue().GetValues()
	if len(records) != 2 || records[1].GetListValue().GetValues()[1].GetStringValue() != "2,3" {
		t.Errorf("Header None: %v", records)
	}

	// What Append CSV writes, Read CSV reads back.
	places := mustValue(t, []any{map[string]any{"name": "Toko, Kopi", "lat": -6.19}, map[string]any{"name": "Shan"}})
	if err := appendCSV(fileCall(data, "append-csv", places, "Path", "round.csv", "Separator", "Tab")); err != nil {
		t.Fatal(err)
	}
	c = fileCall(data, "read-csv", nil, "Path", "round.csv")
	if err := readCSV(c); err != nil {
		t.Fatal(err)
	}
	records = c.outputs["Records"].(*structpb.Value).GetListValue().GetValues()
	if len(records) != 2 || records[0].GetStructValue().GetFields()["name"].GetStringValue() != "Toko, Kopi" ||
		records[0].GetStructValue().GetFields()["lat"].GetStringValue() != "-6.19" {
		t.Errorf("round trip: %v", records)
	}
	if err := readCSV(fileCall(data, "read-csv", nil, "Path", "gone.csv")); err == nil {
		t.Error("missing file: no error")
	}
	if got := strconv.Quote(string(sniff([]byte("a\tb;c\td\n1,2")))); got != `"\t"` {
		t.Errorf("sniff = %s", got)
	}
}
