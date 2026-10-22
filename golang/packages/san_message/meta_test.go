package san_message

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSlotFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "topics.meta")
	sf, err := openSlotFile(path, 4*topicSlotSize, topicSlotSize, tUsed)
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(path); st.Size() != 4*topicSlotSize {
		t.Fatalf("file is %d bytes, want %d", st.Size(), 4*topicSlotSize)
	}
	for i := range sf.slots() {
		if got := sf.free(); got != i {
			t.Fatalf("free = %d, want %d", got, i)
		}
		s := sf.slot(i)
		putKey(s, tKeyLen, tKey, "topic")
		s[tUsed] = 1
	}
	if got := sf.free(); got != -1 {
		t.Fatalf("free on a full file = %d, want -1", got)
	}
	if err := sf.close(); err != nil {
		t.Fatal(err)
	}

	sf, err = openSlotFile(path, 4*topicSlotSize, topicSlotSize, tUsed)
	if err != nil {
		t.Fatal(err)
	}
	if got := getKey(sf.slot(3), tKeyLen, tKey); got != "topic" {
		t.Fatalf("key after reopen = %q", got)
	}
	sf.close()

	if _, err := openSlotFile(path, 8*topicSlotSize, topicSlotSize, tUsed); err == nil {
		t.Fatal("opening with another size worked, want an error")
	}
}
