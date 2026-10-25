package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"
)

// glob runs Glob (the user's ask, 2026-10-25): the files matching Pattern,
// placed as File Path places a name, sorted, with /. Each fires once per
// file, its branch run to the end with Item that file, before the next;
// then Done. Files, First and Count are set before the loop, so its steps
// can read them too.
func glob(c *call) error {
	raw := strings.TrimSpace(c.text("Pattern"))
	if raw == "" {
		return fmt.Errorf("Pattern %w", errEmpty)
	}
	full, err := place(c, "Pattern", raw)
	if err != nil {
		return err
	}
	files, err := globFiles(full)
	if err != nil {
		return fmt.Errorf("Pattern %q: %w", raw, err)
	}
	list := &structpb.ListValue{}
	for _, f := range files {
		list.Values = append(list.Values, structpb.NewStringValue(f))
	}
	first := ""
	if len(files) > 0 {
		first = files[0]
	}
	c.outputs["Files"] = structpb.NewListValue(list)
	c.outputs["First"] = stringValue(first)
	c.outputs["Count"] = stringValue(strconv.Itoa(len(files)))
	c.outputs["Item"] = stringValue("")
	for _, f := range files {
		if c.each == nil {
			break
		}
		c.outputs["Item"] = stringValue(f)
		if err := c.each("Each"); err != nil {
			return err
		}
	}
	c.fire("Done")
	return nil
}

// globFiles lists the files (not folders) matching pattern, an absolute
// path with /: its segments up to the first with * ? or [ are the folder
// searched, and each later one matches one name as path.Match does, but
// ** matches any number of folders. A pattern without them is the file
// itself, when it is there.
func globFiles(pattern string) ([]string, error) {
	segs := strings.Split(pattern, "/")
	i := slices.IndexFunc(segs, func(s string) bool { return strings.ContainsAny(s, "*?[") })
	if i < 0 {
		if fi, err := os.Stat(filepath.FromSlash(pattern)); err == nil && !fi.IsDir() {
			return []string{pattern}, nil
		}
		return nil, nil
	}
	rest := segs[i:]
	for _, s := range rest {
		if _, err := path.Match(s, ""); s != "**" && err != nil {
			return nil, fmt.Errorf("%q is not a pattern", s)
		}
	}
	root := strings.Join(segs[:i], "/")
	if root == "" {
		root = "/"
	}
	if fi, err := os.Stat(filepath.FromSlash(root)); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("the folder %s is not there", root)
	}
	deep := slices.Contains(rest, "**")
	var out []string
	err := filepath.WalkDir(filepath.FromSlash(root), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // a folder that cannot be read is skipped
		}
		rel, err := filepath.Rel(filepath.FromSlash(root), p)
		if err != nil || rel == "." {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if d.IsDir() {
			if !deep && len(parts) >= len(rest) {
				return fs.SkipDir
			}
			return nil
		}
		if matchSegs(rest, parts) {
			out = append(out, path.Join(root, filepath.ToSlash(rel)))
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.SkipAll) {
		return nil, err
	}
	slices.Sort(out)
	return out, nil
}

// matchSegs matches a path's names against a pattern's segments.
func matchSegs(pat, name []string) bool {
	if len(pat) == 0 {
		return len(name) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(name); i++ {
			if matchSegs(pat[1:], name[i:]) {
				return true
			}
		}
		return false
	}
	if len(name) == 0 {
		return false
	}
	ok, err := path.Match(pat[0], name[0])
	return err == nil && ok && matchSegs(pat[1:], name[1:])
}
