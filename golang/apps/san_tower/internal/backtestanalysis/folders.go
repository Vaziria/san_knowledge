package backtestanalysis

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Folders lists a folder's subfolders for the page's folder picker (the
// browser cannot give a real path). An empty path lists the drives on
// Windows, or the root elsewhere. Hidden folders are left out.
func Folders(path string) (*v1.BacktestListFoldersResponse, error) {
	home, _ := os.UserHomeDir()
	res := &v1.BacktestListFoldersResponse{Home: home}
	if strings.TrimSpace(path) == "" {
		if runtime.GOOS == "windows" {
			for c := 'A'; c <= 'Z'; c++ {
				d := string(c) + `:\`
				if st, err := os.Stat(d); err == nil && st.IsDir() {
					res.Folders = append(res.Folders, &v1.BacktestFolderEntry{Name: string(c) + ":", Path: d})
				}
			}
			return res, nil
		}
		path = "/"
	}
	dir, err := Clean(path)
	if err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	res.Path, res.Report = dir, IsReport(dir)
	if parent := filepath.Dir(dir); parent != dir {
		res.Parent = parent
	} else if runtime.GOOS == "windows" {
		res.Parent = "" // a drive's top: up goes to the drives
	}
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") {
			continue
		}
		p := filepath.Join(dir, name)
		if !e.IsDir() {
			// a link or junction to a folder counts as one
			if e.Type()&os.ModeSymlink == 0 {
				continue
			}
			if st, err := os.Stat(p); err != nil || !st.IsDir() {
				continue
			}
		}
		res.Folders = append(res.Folders, &v1.BacktestFolderEntry{Name: name, Path: p, Report: IsReport(p)})
	}
	sort.Slice(res.Folders, func(i, j int) bool {
		return strings.ToLower(res.Folders[i].Name) < strings.ToLower(res.Folders[j].Name)
	})
	return res, nil
}
