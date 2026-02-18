package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/ttacon/chalk"
)

var useTmux bool
var monitorInterval int

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor system for changes",
	Long:  `Continuously checks for new processes, socket connections, and file changes.`,
	Run: func(cmd *cobra.Command, args []string) {
		if useTmux {
			// Check if tmux is installed
			if _, err := exec.LookPath("tmux"); err != nil {
				fmt.Println(NewMessage(chalk.Yellow, "Tmux not found. Attempting to install..."))
				// Try dnf (Fedora) first
				if err := RunCommand("dnf", "install", "-y", "tmux"); err != nil {
					// Try apt (Debian/Ubuntu)
					if err := RunCommand("apt-get", "install", "-y", "tmux"); err != nil {
						fmt.Println(NewMessage(chalk.Red, "Failed to install tmux. Please install manually."))
						return
					}
				}
			}

			// Re-launch self in tmux
			exe, err := os.Executable()
			if err != nil {
				fmt.Println(NewMessage(chalk.Red, "Failed to find executable path: "+err.Error()))
				return
			}

			// We need to call "monitor" without "--tmux" inside the window to avoid loop
			// tmux new-window -n "Monitor" "{exe} monitor"
			fmt.Println(NewMessage(chalk.Green, "Launching monitor in new tmux window..."))
			err = RunCommand("tmux", "new-window", "-n", "Monitor", exe, "monitor")
			if err != nil {
				// If no session exists, maybe try new-session?
				// "no server running on /tmp/tmux-..."
				// Usually persistent CCDC envs have tmux running. If not:
				fmt.Println(NewMessage(chalk.Yellow, "Failed to create window (is tmux running?). Trying new session..."))
				err = RunCommand("tmux", "new-session", "-d", "-s", "defense", "-n", "Monitor", exe, "monitor")
				if err != nil {
					fmt.Println(NewMessage(chalk.Red, "Failed to launch tmux: "+err.Error()))
				} else {
					fmt.Println(NewMessage(chalk.Green, "Started new tmux session 'defense' with monitor."))
					fmt.Println(NewMessage(chalk.Blue, "Attach with: tmux attach -t defense"))
				}
			}
			return
		}

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
	monitorCmd.Flags().BoolVarP(&useTmux, "tmux", "t", false, "Install tmux and run monitor in a new window")
	monitorCmd.Flags().IntVarP(&monitorInterval, "interval", "i", 2, "Monitoring interval in seconds")
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
	// Run ss -tunap to show TCP/UDP, numeric, all, processes
	// We specifically look for ESTAB connections to show active sessions
	out, err := exec.Command("ss", "-tunap").Output()
	if err != nil {
		// Fallback or just return silent?
		// If ss isn't there, maybe netstat? keeping it simple for now.
		return
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "ESTAB") {
			// Print Established connections
			// Highlight potentially dangerous ports?
			// For now, just print valid established connections
			fmt.Println(NewMessage(chalk.Magenta, "Active Connection: "+strings.TrimSpace(line)))
		}
	}
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
