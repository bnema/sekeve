package clipboard

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Copy writes value to the system clipboard via wl-copy or xclip.
func Copy(ctx context.Context, value string) error {
	cmd, name := cmd(ctx)
	cmd.Stdin = strings.NewReader(value)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func cmd(ctx context.Context) (*exec.Cmd, string) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return exec.CommandContext(ctx, "wl-copy"), "wl-copy"
	}
	return exec.CommandContext(ctx, "xclip", "-selection", "clipboard"), "xclip"
}
