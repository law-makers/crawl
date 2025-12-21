package config

import "github.com/spf13/cobra"

// RegisterFlags registers common CLI flags on the provided root command
func RegisterFlags(cmd *cobra.Command) {
	if cmd == nil {
		return
	}

	cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable debug logging")
	cmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress all output except errors")
	cmd.PersistentFlags().Bool("json", false, "Output in JSON format only")
	cmd.PersistentFlags().String("proxy", "", "Set HTTP/SOCKS5 proxy (e.g., http://localhost:8080)")
	cmd.PersistentFlags().String("timeout", "30s", "Set hard timeout for requests")
	cmd.PersistentFlags().String("user-agent", "", "Custom user agent string")
	cmd.PersistentFlags().String("config", "", "Path to configuration file (optional)")
}
