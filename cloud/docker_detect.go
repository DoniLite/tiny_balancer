package cloud

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/moby/moby/client"
)

func pingOK(cli *client.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	_, err := cli.Ping(ctx)
	return err
}

func dockerHostCandidates() []string {
	var out []string
	switch runtime.GOOS {
	case "windows":
		out = append(out, "npipe:////./pipe/docker_engine")
	default:
		if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
			out = append(out, "unix://"+filepath.Join(xdg, "docker.sock"))
		}
		if uid := os.Getuid(); uid > 0 {
			out = append(out, fmt.Sprintf("unix:///run/user/%d/docker.sock", uid))
		}
		out = append(out, "unix:///var/run/docker.sock")
	}
	// optional TCP (off by default, but some users enable it)
	out = append(out, "tcp://127.0.0.1:2375")
	return out
}
