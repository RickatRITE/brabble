package tray

import (
	"brabble/internal/config"

	"github.com/spf13/cobra"
)

// NewTrayCmd returns the cobra command for the system tray.
func NewTrayCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "tray",
		Short: "Launch system tray icon (Windows)",
		Long:  "Starts a system tray icon that shows daemon status, recent transcripts, and provides quick controls.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return err
			}
			Run(cfg)
			return nil
		},
	}
}
