// Command san_radio streams the .wav files in a folder around the clock over
// a looping background video, and greets the live chat on screen.
//
//	SAN_RADIO_KEY=xxxx-xxxx san_radio -video loop.mp4 -music music -chat @yourchannel
//	san_radio -video loop.mp4 -music music -out test.flv
//
// It runs until Ctrl+C or SIGTERM, starting ffmpeg again whenever it stops.
// Logs go to stderr.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/wargasipil/san_radio"
	"gopkg.in/yaml.v3"
)

// version is stamped by the build scripts.
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "san_radio:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("san_radio", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		configFile  = fs.String("config", "", "YAML file with stream_id (the stream key), stream_url and youtube_channel, e.g. assets/channels.yaml; flags given as well win")
		video       = fs.String("video", "", "background video, looped (required); render it so its end runs into its start")
		music       = fs.String("music", "music", "folder of .wav files, played in name order, again and again")
		url         = fs.String("url", "rtmp://a.rtmp.youtube.com/live2", "RTMP server; the stream key is added to it")
		keyFile     = fs.String("key-file", "", "file holding the stream key (default: $SAN_RADIO_KEY)")
		out         = fs.String("out", "", "send to this URL or file instead of -url and the key, e.g. test.flv")
		chat        = fs.String("chat", "", "channel whose live chat is greeted on screen, e.g. @yourchannel")
		poll        = fs.Duration("poll", 0, "read the chat at least this often, e.g. 3s (0: as YouTube asks, about every 10s); YouTube's pace again for 10 minutes after it says too many requests")
		blockedFile = fs.String("blocked", "", "file of words, one per line, that keep a comment off the screen")
		size        = fs.String("size", "1280x720", "video size")
		fps         = fs.Int("fps", 30, "frames per second")
		bitrate     = fs.String("bitrate", "2500k", "video bitrate")
		encoder     = fs.String("encoder", "auto", "H.264 encoder: auto (a Raspberry Pi's hardware encoder when it works, else libx264) or an ffmpeg encoder name")
		preset      = fs.String("preset", "", "libx264 preset (default: veryfast; ultrafast on ARM)")
		decoder     = fs.String("decoder", "auto", "decoder for the video: auto (a Raspberry Pi's hardware decoder when it works, else software), software, or an ffmpeg decoder name")
		textDir     = fs.String("text-dir", defaultTextDir(), "folder for the terminal's files, which ffmpeg reads every frame")
		ffmpeg      = fs.String("ffmpeg", "ffmpeg", "ffmpeg to run")
		showVersion = fs.Bool("version", false, "print the version and exit")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: san_radio -video <file> [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, "san_radio", version)
		return nil
	}
	if fs.NArg() > 0 {
		fs.Usage()
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if *video == "" {
		fs.Usage()
		return errors.New("-video is required")
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	configKey := ""
	if *configFile != "" {
		cfg, err := readConfig(*configFile)
		if err != nil {
			return err
		}
		if !set["url"] && cfg.StreamURL != "" {
			*url = cfg.StreamURL
		}
		if !set["chat"] && cfg.YoutubeChannel != "" {
			*chat = cfg.YoutubeChannel
		}
		configKey = cfg.StreamID
	}

	c := san_radio.Config{
		FFmpeg: *ffmpeg, Video: *video, Music: *music,
		FPS: *fps, Encoder: *encoder, Preset: *preset, Decoder: *decoder,
		TextDir: *textDir,
		Chat:    *chat, Poll: *poll,
	}
	var err error
	if c.Width, c.Height, err = parseSize(*size); err != nil {
		return err
	}
	if c.Bitrate, err = parseBitrate(*bitrate); err != nil {
		return err
	}
	if c.FPS < 1 || c.FPS > 60 {
		return fmt.Errorf("-fps %d: want 1 to 60", c.FPS)
	}
	if *blockedFile != "" {
		if c.Blocked, err = readWords(*blockedFile); err != nil {
			return err
		}
	}
	if *out != "" {
		c.Output = *out
	} else {
		key, err := streamKey(*keyFile, configKey)
		if err != nil {
			return err
		}
		c.Output = strings.TrimRight(*url, "/") + "/" + key
		c.Secret = key
	}

	// Under systemd the journal stamps the time already.
	flags := log.LstdFlags
	if os.Getenv("INVOCATION_ID") != "" {
		flags = 0
	}
	c.Log = log.New(stderr, "", flags)
	c.Log.Printf("san_radio %s: music from %s, video %s", version, c.Music, c.Video)
	return san_radio.Run(ctx, c)
}

// channelConfig is a settings file such as assets/channels.yaml:
//
//	stream_id: xxxx-xxxx-xxxx-xxxx-xxxx    # the stream key
//	stream_url: rtmp://a.rtmp.youtube.com/live2
//	youtube_channel: "@yourchannel"        # whose live chat is greeted
type channelConfig struct {
	StreamID       string `yaml:"stream_id"`
	StreamURL      string `yaml:"stream_url"`
	YoutubeChannel string `yaml:"youtube_channel"`
}

func readConfig(file string) (channelConfig, error) {
	var cfg channelConfig
	b, err := os.ReadFile(file)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("%s: %w", file, err)
	}
	return cfg, nil
}

// streamKey is the key from -key-file, else from -config, else from
// $SAN_RADIO_KEY.
func streamKey(file, fromConfig string) (string, error) {
	key := os.Getenv("SAN_RADIO_KEY")
	if fromConfig != "" {
		key = fromConfig
	}
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		key = string(b)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errors.New("no stream key: set $SAN_RADIO_KEY or -key-file (or use -out to test)")
	}
	return key, nil
}

func parseSize(s string) (w, h int, err error) {
	ws, hs, ok := strings.Cut(strings.ToLower(s), "x")
	if ok {
		w, err = strconv.Atoi(ws)
		if err == nil {
			h, err = strconv.Atoi(hs)
		}
	}
	// The stream's yuv420p wants even sides.
	if !ok || err != nil || w < 16 || h < 16 || w%2 != 0 || h%2 != 0 {
		return 0, 0, fmt.Errorf("-size %q: want an even WIDTHxHEIGHT such as 1280x720", s)
	}
	return w, h, nil
}

// parseBitrate reads "2500k", "2.5M" or plain bits per second.
func parseBitrate(s string) (int, error) {
	mult := 1.0
	num := strings.TrimSpace(s)
	switch {
	case strings.HasSuffix(strings.ToLower(num), "k"):
		mult, num = 1e3, num[:len(num)-1]
	case strings.HasSuffix(strings.ToLower(num), "m"):
		mult, num = 1e6, num[:len(num)-1]
	}
	f, err := strconv.ParseFloat(num, 64)
	if err != nil || f*mult < 100e3 || f*mult > 50e6 {
		return 0, fmt.Errorf("-bitrate %q: want e.g. 2500k, between 100k and 50M", s)
	}
	return int(f * mult), nil
}

// readWords reads one word per line, lowercased; empty lines and lines
// starting with "#" are skipped.
func readWords(file string) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var words []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		w := strings.ToLower(strings.TrimSpace(sc.Text()))
		if w != "" && !strings.HasPrefix(w, "#") {
			words = append(words, w)
		}
	}
	return words, sc.Err()
}

// defaultTextDir is /dev/shm where there is one: the greeting files are read
// 60 times a second, and written on every greeting, which an SD card is
// better spared.
func defaultTextDir() string {
	if fi, err := os.Stat("/dev/shm"); err == nil && fi.IsDir() {
		return "/dev/shm"
	}
	return os.TempDir()
}
