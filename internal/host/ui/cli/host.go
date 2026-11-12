package cli

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	hostdomain "github.com/90p-labs/harness/internal/host/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	commandHost         = "host"
	commandInstall      = "install"
	commandUninstall    = "uninstall"
	commandCapabilities = "capabilities"
	flagHost            = "host"
	answerYes           = "yes"
	answerNo            = "no"
)

var ErrUnknownHost = errors.New("unknown host")

var knownHosts = []shareddomain.Host{shareddomain.HostClaude, shareddomain.HostCodex}

// PluginWarning is printed when the harness plugin of the host already runs the hooks that install writes.
const PluginWarning = "warning: the 90p plugin also runs the harness hooks, so each hook runs twice; " +
	"remove them from this file with harness host uninstall --host claude"

// HostHandlers are the host use cases. An empty host asks each handler to detect the host of the worktree.
type HostHandlers struct {
	Install func(ctx context.Context, host shareddomain.Host) (string, error)
	// Plugin reports whether the harness plugin of the host already runs the hooks.
	Plugin func(ctx context.Context, host shareddomain.Host) (bool, error)
	// Uninstall removes the harness hook entries from the host config and returns the path and the count removed.
	Uninstall func(ctx context.Context, host shareddomain.Host) (string, int, error)
	Codecs    []hostdomain.Codec
}

type HostConnect func(ctx context.Context) (HostHandlers, error)

func NewHostCommand(connect HostConnect) *cobra.Command {
	cmd := &cobra.Command{Use: commandHost, Short: "Work with the agent hosts"}
	cmd.AddCommand(installCommand(connect), uninstallCommand(connect), capabilitiesCommand(connect))
	return cmd
}

// optionalHost returns the --host value, or an empty host when the flag is not set.
func optionalHost(cmd *cobra.Command) (shareddomain.Host, error) {
	if !cmd.Flags().Changed(flagHost) {
		return "", nil
	}
	return hostFlag(cmd)
}

func uninstallCommand(connect HostConnect) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandUninstall,
		Short: "Remove the harness hooks from the host config of this worktree and keep the others; with no --host, detect the host",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			host, err := optionalHost(cmd)
			if err != nil {
				return err
			}
			handlers, err := connect(cmd.Context())
			if err != nil {
				return err
			}
			path, removed, err := handlers.Uninstall(cmd.Context(), host)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %d harness hooks from %s\n", removed, path)
			return nil
		},
	}
	addHostFlag(cmd)
	return cmd
}

func installCommand(connect HostConnect) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandInstall,
		Short: "Write the harness hooks into the host config of this worktree; with no --host, detect the host",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			host, err := optionalHost(cmd)
			if err != nil {
				return err
			}
			handlers, err := connect(cmd.Context())
			if err != nil {
				return err
			}
			plugin, err := handlers.Plugin(cmd.Context(), host)
			if err != nil {
				return err
			}
			path, err := handlers.Install(cmd.Context(), host)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			if plugin {
				fmt.Fprintln(cmd.ErrOrStderr(), PluginWarning)
			}
			return nil
		},
	}
	addHostFlag(cmd)
	return cmd
}

func capabilitiesCommand(connect HostConnect) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandCapabilities,
		Short: "Show the events that the host sends",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			host, err := hostFlag(cmd)
			if err != nil {
				return err
			}
			handlers, err := connect(cmd.Context())
			if err != nil {
				return err
			}
			codec, err := codecOf(handlers.Codecs, host)
			if err != nil {
				return err
			}
			missing := hostdomain.MissingEvents(codec)
			for _, event := range hostdomain.AllEvents {
				answer := answerYes
				if slices.Contains(missing, event) {
					answer = answerNo
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", event, answer)
			}
			return nil
		},
	}
	addHostFlag(cmd)
	_ = cmd.MarkFlagRequired(flagHost)
	return cmd
}

func addHostFlag(cmd *cobra.Command) {
	cmd.Flags().String(flagHost, "", "the host: claude or codex")
}

func hostFlag(cmd *cobra.Command) (shareddomain.Host, error) {
	value, err := cmd.Flags().GetString(flagHost)
	if err != nil {
		return "", err
	}
	host := shareddomain.Host(value)
	if !slices.Contains(knownHosts, host) {
		return "", fmt.Errorf("%w: %q", ErrUnknownHost, value)
	}
	return host, nil
}

func codecOf(codecs []hostdomain.Codec, host shareddomain.Host) (hostdomain.Codec, error) {
	for _, codec := range codecs {
		if codec.Host() == host {
			return codec, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknownHost, host)
}
