// Command transcript is a scratch web app for testing streaming speech-to-text
// with Vosk: speak into the browser, watch partial and final results arrive.
//
// Audio is captured in the page at 16 kHz mono, streamed as raw signed 16-bit
// PCM over a websocket, and fed straight to a Vosk recognizer. Vosk does its
// own endpointing, so a "final" here is an utterance boundary it chose.
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
)

//go:embed static
var staticFS embed.FS

func main() {
	addr := flag.String("addr", "127.0.0.1:7575", "listen address")
	modelsDir := flag.String("models", "models", "directory holding unpacked Vosk models, one per subdirectory")
	open := flag.Bool("open", true, "open a browser on start")
	flag.Parse()

	abs, err := filepath.Abs(*modelsDir)
	if err != nil {
		log.Fatalf("models dir: %v", err)
	}
	names, err := listModels(abs)
	if err != nil {
		log.Fatalf("models dir: %v", err)
	}
	if len(names) == 0 {
		log.Fatalf("no models found in %s - run setup.ps1 to download them", abs)
	}
	log.Printf("models in %s: %v", abs, names)

	hub := &hub{dir: abs}
	defer hub.close()

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"models": mustList(abs)})
	})
	mux.HandleFunc("/ws", hub.serveWS)

	url := "http://" + *addr + "/"
	log.Printf("transcript listening on %s", url)
	if *open {
		go browse(url)
	}
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

// listModels returns the subdirectories that hold an unpacked Vosk model.
func listModels(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s does not exist", dir)
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && validModelDir(filepath.Join(dir, e.Name())) == nil {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// validModelDir checks the layout before anything is handed to Vosk.
//
// This matters more than it looks: vosk_model_new returns NULL for a bad path
// and the Go binding reports no error for it, so the NULL reaches
// vosk_recognizer_new and takes the whole process down with an access
// violation. Every load has to be gated on a check of our own.
func validModelDir(path string) error {
	if _, err := os.Stat(filepath.Join(path, "conf")); err != nil {
		return fmt.Errorf("no conf/ directory in %s", path)
	}
	for _, am := range []string{"am", "am-bin"} {
		if _, err := os.Stat(filepath.Join(path, am)); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no am/ or am-bin/ directory in %s", path)
}

func mustList(dir string) []string {
	names, err := listModels(dir)
	if err != nil {
		return nil
	}
	return names
}

func browse(url string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		exec.Command("open", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}
