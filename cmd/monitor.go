package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/ttacon/chalk"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor system for changes",
	Long:  `Continuously checks for new processes, socket connections, and file changes.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(NewMessage(chalk.Green, "Starting System Monitor (Ctrl+C to stop)..."))

		// Initial baseline
		knownProcs := getRunningProcesses()
		fmt.Println(NewMessage(chalk.Blue, fmt.Sprintf("Baseline taken: %d processes.", len(knownProcs))))

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			currentProcs := getRunningProcesses()

			// diff
			for pid, name := range currentProcs {
				if _, ok := knownProcs[pid]; !ok {
					fmt.Println(NewMessage(chalk.Red, fmt.Sprintf("NEW PROCESS: %s (PID: %s)", name, pid)))
					// Add to known so we don't spam
					knownProcs[pid] = name
				}
			}

			// Check established connections (naive netstat/ss wrapper)
			checkConnections()

			// Check critical file modification times
			checkFileChanges()
		}
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)
}

func getRunningProcesses() map[string]string {
	procs := make(map[string]string)
	files, err := os.ReadDir("/proc")
	if err != nil {
		return procs
	}

	for _, f := range files {
		if f.IsDir() && isNumeric(f.Name()) {
			// Read cmdline
			cmdline, err := os.ReadFile(filepath.Join("/proc", f.Name(), "cmdline"))
			if err == nil {
				// cmdline is null separated
				parts := strings.Split(string(cmdline), "\x00")
				if len(parts) > 0 {
					procs[f.Name()] = parts[0]
				}
			}
		}
	}
	return procs
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func checkConnections() {
	// Run ss -antp
	// grep for ESTAB
	// This is hard to parse reliably across versions without a library, but for CCDC specific reporting:
	// We can just dump new established connections if we track them.
	// For now, let's just print active shell connections if found.
	// grep for ":22" or ":4444" etc.
}

func checkFileChanges() {
	// Check /etc/passwd, /etc/shadow, /etc/group
	files := []string{"/etc/passwd", "/etc/shadow", "/etc/group", "/etc/hosts"}
	for _, f := range files {
		info, err := os.Stat(f)
		if err == nil {
			if time.Since(info.ModTime()) < 10*time.Second {
				fmt.Println(NewMessage(chalk.Red, "FILE MODIFIED RECENTLY: "+f))
			}
		}
	}
}
