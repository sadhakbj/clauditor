// main.go - Entry point for claude-usage CLI (Go implementation).
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagPort      int
	flagNoBrowser bool
)

var rootCmd = &cobra.Command{
	Use:   "claude-usage",
	Short: "Claude Code usage dashboard",
	Long:  "Track and visualize Claude Code token usage from your local transcript files.",
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan JSONL files and update the database",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmdScan()
		return nil
	},
}

var todayCmd = &cobra.Command{
	Use:   "today",
	Short: "Show today's usage summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmdToday()
		return nil
	},
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show all-time statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmdStats()
		return nil
	},
}

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Scan + start local web dashboard",
	Long:  "Runs a scan then starts a local HTTP dashboard at the given port.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmdDashboard(flagPort, flagNoBrowser)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", dbPath, "Path to SQLite database")
	rootCmd.PersistentFlags().StringVar(&projectsDir, "dir", projectsDir, "Path to Claude projects directory")

	dashboardCmd.Flags().IntVar(&flagPort, "port", 8080, "Port for the dashboard server")
	dashboardCmd.Flags().BoolVar(&flagNoBrowser, "no-browser", false, "Don't open browser automatically")

	rootCmd.AddCommand(scanCmd, todayCmd, statsCmd, dashboardCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
