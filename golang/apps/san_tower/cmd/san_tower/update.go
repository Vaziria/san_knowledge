package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/urfave/cli/v3"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/update"
)

// ----------------------------------------------------------- update ---

func updateCommand() *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "install the newest San Tower release over this one, then restart the running tower",
		Description: "Asks GitHub for the latest release (" + update.Repo + "), downloads its bundle for this\n" +
			"system and processor, checks that the new san_tower runs, and puts it and the files beside it\n" +
			"(README.txt, start.bat, san_remote_extension/) in this san_tower's folder. Then the tower at\n" +
			"--server, if it runs from this file, restarts into the new version; with runs in progress it\n" +
			"waits for you: `san_tower restart` once they end, or --force.",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "check", Usage: "only say whether a newer release is out"},
			&cli.BoolFlag{Name: "force", Usage: "install over a local build or the same version, and restart even with runs in progress"},
		},
		Action: runUpdate,
	}
}

func runUpdate(ctx context.Context, cmd *cli.Command) error {
	out := cmd.Root().Writer
	force := cmd.Bool("force")
	hc := &http.Client{Timeout: 15 * time.Minute}
	rel, err := update.Latest(ctx, hc)
	if err != nil {
		return err
	}
	latest, ok := update.Parse(rel.Tag)
	if !ok {
		return fmt.Errorf("the latest release's tag %q is not a version", rel.Tag)
	}
	cur, release := update.Parse(version)
	switch {
	case !release && !force:
		return fmt.Errorf("this san_tower is a local build (%s), not a release: build it again, or --force to install %s over it", version, rel.Tag)
	case release && !cur.Less(latest) && !force:
		fmt.Fprintf(out, "san_tower %s is the newest release\n", version)
		return nil
	case cmd.Bool("check"):
		fmt.Fprintf(out, "%s is out (this is %s): `san_tower update` installs it\n%s\n", rel.Tag, version, rel.URL)
		return nil
	}
	a, err := rel.AssetFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "downloading %s (%.1f MB)\n", a.Name, float64(a.Size)/(1<<20))
	if err := update.Install(ctx, hc, a, rel.Tag, exe); err != nil {
		return err
	}
	fmt.Fprintf(out, "installed %s in %s\n", rel.Tag, filepath.Dir(exe))
	return restartTower(ctx, cmd, exe, rel.Tag, force)
}

// ---------------------------------------------------------- restart ---

func restartCommand() *cli.Command {
	return &cli.Command{
		Name:  "restart",
		Usage: "restart the running tower (after `san_tower update`), once no run is in progress",
		Flags: []cli.Flag{&cli.BoolFlag{Name: "force", Usage: "restart even with runs in progress, cutting them short"}},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return restartTower(ctx, cmd, "", "", cmd.Bool("force"))
		},
	}
}

// restartTower asks the tower at --server to restart, when it runs from exe
// (any tower when exe is empty), and waits until it answers again, as
// version want when given.
func restartTower(ctx context.Context, cmd *cli.Command, exe, want string, force bool) error {
	out, url := cmd.Root().Writer, cmd.String("server")
	c := towerv1connect.NewTowerServiceClient(&http.Client{Timeout: 10 * time.Second}, url)
	st, err := c.GetStatus(ctx, connect.NewRequest(&v1.GetStatusRequest{}))
	if err != nil {
		if exe != "" {
			fmt.Fprintf(out, "no tower answers at %s: the next `san_tower serve` runs %s\n", url, want)
			return nil
		}
		return fmt.Errorf("no tower at %s: start it with `san_tower serve`", url)
	}
	if exe != "" && !sameFile(st.Msg.GetExecutable(), exe) {
		fmt.Fprintf(out, "the tower at %s runs from %s, not from this san_tower: restart it yourself\n", url, st.Msg.GetExecutable())
		return nil
	}
	if _, err := c.Restart(ctx, connect.NewRequest(&v1.RestartRequest{Force: force})); err != nil {
		var ce *connect.Error
		if errors.As(err, &ce) && ce.Code() == connect.CodeFailedPrecondition {
			return fmt.Errorf("the tower did not restart: %s; `san_tower restart` once they end, or --force", ce.Message())
		}
		return err
	}
	fmt.Fprintf(out, "restarting the tower at %s\n", url)
	before := st.Msg.GetStartedAt().AsTime()
	for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); {
		time.Sleep(time.Second)
		st, err := c.GetStatus(ctx, connect.NewRequest(&v1.GetStatusRequest{}))
		if err != nil || !st.Msg.GetStartedAt().AsTime().After(before) {
			continue
		}
		if want != "" && st.Msg.GetVersion() != want {
			return fmt.Errorf("the tower restarted as %s, not %s", st.Msg.GetVersion(), want)
		}
		fmt.Fprintf(out, "the tower runs %s\n", st.Msg.GetVersion())
		return nil
	}
	return fmt.Errorf("the tower at %s does not answer a minute after restarting: look at its log", url)
}

// sameFile says whether paths a and b are one file.
func sameFile(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	fa, errA := os.Stat(a)
	fb, errB := os.Stat(b)
	if errA == nil && errB == nil {
		return os.SameFile(fa, fb)
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
