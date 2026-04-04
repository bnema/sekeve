package client

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/bnema/sekeve/internal/adapters/cli/cliconfig"
	"github.com/bnema/sekeve/internal/adapters/gui"
	"github.com/bnema/sekeve/internal/adapters/notification"
	"github.com/spf13/cobra"
)

// WithGUI wraps a cobra command to wire the GUIAdapter before execution.
// This keeps the gui import in the client package, away from root/server.
func WithGUI(cmd *cobra.Command) *cobra.Command {
	existing := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		ensureLayerShell()
		if existing != nil {
			if err := existing(cmd, args); err != nil {
				return err
			}
		}
		ctx := cmd.Context()
		guiAdapter := gui.NewGUIAdapter()
		ctx = cliconfig.WithPINPrompt(ctx, guiAdapter)
		ctx = cliconfig.WithGUI(ctx, guiAdapter)

		notifier, err := notification.NewDBus()
		if err != nil {
			notifier = notification.NewNoop()
		}
		cobra.OnFinalize(func() {
			if err := notifier.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "sekeve: notification cleanup: %v\n", err)
			}
		})
		ctx = cliconfig.WithNotify(ctx, notifier)

		cmd.SetContext(ctx)
		return nil
	}
	return cmd
}

// ensureLayerShell re-execs the current process with LD_PRELOAD set for
// gtk4-layer-shell if running on Wayland without it. The library's GDK
// backend hook only works when loaded before GTK initializes, which
// requires LD_PRELOAD rather than dlopen.
func ensureLayerShell() {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return
	}
	if strings.Contains(os.Getenv("LD_PRELOAD"), "libgtk4-layer-shell") {
		return
	}

	libPath := findLayerShellLib()
	if libPath == "" {
		return
	}

	preload := os.Getenv("LD_PRELOAD")
	if preload != "" {
		preload += ":"
	}
	preload += libPath

	exe, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return
	}

	env := os.Environ()
	found := false
	for i, e := range env {
		if strings.HasPrefix(e, "LD_PRELOAD=") {
			env[i] = "LD_PRELOAD=" + preload
			found = true
			break
		}
	}
	if !found {
		env = append(env, "LD_PRELOAD="+preload)
	}

	// Replace current process; on success this never returns.
	_ = syscall.Exec(exe, os.Args, env)
}

func findLayerShellLib() string {
	for _, p := range []string{
		"/usr/lib/libgtk4-layer-shell.so.0",
		"/usr/lib64/libgtk4-layer-shell.so.0",
		"/usr/lib/x86_64-linux-gnu/libgtk4-layer-shell.so.0",
		"/usr/lib/aarch64-linux-gnu/libgtk4-layer-shell.so.0",
		"/app/lib/libgtk4-layer-shell.so.0",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
