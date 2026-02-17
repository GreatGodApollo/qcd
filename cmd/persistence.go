package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ttacon/chalk"
)

var autoRemove bool

var persistenceCmd = &cobra.Command{
	Use:   "persistence",
	Short: "Check for and remove common persistence mechanisms",
	Long:  `Scans system for persistence mechanisms including cron, systemd, users, startup scripts, and more. Can automatically remove found persistence with --auto.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(NewMessage(chalk.Green, "Starting Persistence Scan..."))

		checkCron()
		checkSystemd()
		checkUsers()
		checkStartup()
		checkPreload()
		checkSUID() // Basic SUID check

		fmt.Println(NewMessage(chalk.Green, "Persistence Scan Complete."))
	},
}

func init() {
	rootCmd.AddCommand(persistenceCmd)
	persistenceCmd.Flags().BoolVarP(&autoRemove, "auto", "a", false, "Automatically attempt to remove/fix found persistence")
}

// --- Cron Checks ---
func checkCron() {
	fmt.Println(NewMessage(chalk.Blue, "Checking Cron Jobs..."))
	dirs := []string{"/var/spool/cron", "/etc/cron.d", "/etc/cron.daily", "/etc/cron.hourly", "/etc/cron.monthly", "/etc/cron.weekly"}
	for _, dir := range dirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue // Skip if can't read
		}
		for _, file := range files {
			path := filepath.Join(dir, file.Name())
			if !file.IsDir() {
				fmt.Println(NewMessage(chalk.Yellow, "Found cron file: "+path))
				scanFileForSuspiciousContent(path)
			}
		}
	}
	// Check /etc/crontab
	scanFileForSuspiciousContent("/etc/crontab")
}

// --- Systemd Checks ---
func checkSystemd() {
	fmt.Println(NewMessage(chalk.Blue, "Checking Systemd Units..."))
	// Simplified check for "odd" service names or modification times could go here.
	// For now, listing services in /etc/systemd/system which are user created
	files, err := os.ReadDir("/etc/systemd/system")
	if err == nil {
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".service") || strings.HasSuffix(file.Name(), ".timer") {
				fmt.Println(NewMessage(chalk.Yellow, "Found local systemd unit: "+file.Name()))
				// If auto-remove, maybe disable? Too risky to auto-disable all.
				// We will just alert for now unless we have a blacklist.
			}
		}
	}
}

// --- User Checks ---
func checkUsers() {
	fmt.Println(NewMessage(chalk.Blue, "Checking Users..."))
	ignoredUsers := viper.GetStringSlice("persistence.ignore_users")
	ignoredMap := make(map[string]bool)
	for _, u := range ignoredUsers {
		ignoredMap[u] = true
	}

	file, err := os.Open("/etc/passwd")
	if err != nil {
		fmt.Println(NewMessage(chalk.Red, "Could not read /etc/passwd"))
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) > 2 {
			uid := parts[2]
			user := parts[0]
			if uid == "0" && !ignoredMap[user] {
				fmt.Println(NewMessage(chalk.Red, "ALERT: Non-ignored user with UID 0 found: "+user))
				if autoRemove {
					fmt.Println(NewMessage(chalk.Yellow, "Auto-locking account: "+user))
					RunCommand("usermod", "-L", user)
				}
			}
		}
	}

	// Check authorized_keys
	keysPath := "/root/.ssh/authorized_keys"
	if _, err := os.Stat(keysPath); err == nil {
		fmt.Println(NewMessage(chalk.Yellow, "Checking /root/.ssh/authorized_keys..."))
		content, _ := os.ReadFile(keysPath)
		if len(content) > 0 {
			fmt.Println(NewMessage(chalk.Red, "Root has authorized_keys entry!"))
			if autoRemove {
				fmt.Println(NewMessage(chalk.Yellow, "Backing up and clearing authorized_keys..."))
				os.Rename(keysPath, keysPath+".bak")
				os.WriteFile(keysPath, []byte(""), 0600)
			}
		}
	}
}

// --- Startup Checks ---
func checkStartup() {
	fmt.Println(NewMessage(chalk.Blue, "Checking Startup Scripts..."))
	files := []string{"/root/.bashrc", "/root/.profile", "/etc/profile", "/etc/bashrc"}
	for _, file := range files {
		scanFileForSuspiciousContent(file)
	}
}

// --- Preload Checks ---
func checkPreload() {
	fmt.Println(NewMessage(chalk.Blue, "Checking LD_PRELOAD..."))
	path := "/etc/ld.so.preload"
	if _, err := os.Stat(path); err == nil {
		fmt.Println(NewMessage(chalk.Red, "ALERT: /etc/ld.so.preload exists!"))
		content, _ := os.ReadFile(path)
		fmt.Println(string(content))
		if autoRemove {
			fmt.Println(NewMessage(chalk.Yellow, "Removing /etc/ld.so.preload..."))
			os.Remove(path)
		}
	}
}

// --- SUID Checks ---
func checkSUID() {
	fmt.Println(NewMessage(chalk.Blue, "Checking common SUID binaries..."))
	// Actual full scan is slow: find / -perm -4000
	// We can do a quick check of common exploitable bins if widely known, or just run the find command
	// user asked for SUID binaries abuse.
	// Let's run a find command limited to /usr/bin and /bin for speed in this tool
	// find /usr/bin /bin -perm -4000
	// This is just a reporter primarily.

	// We will just spawn the find command and let it output to stdout
	RunCommand("find", "/bin", "/usr/bin", "-perm", "-4000")
}

// --- Generic File Scanner ---
func scanFileForSuspiciousContent(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	strContent := string(content)
	// Very basic signature check
	suspicious := []string{"nc -e", "bash -i", "dev/tcp", "curl", "wget", "python -c", "systemctl stop", "iptables", "nft", "systemctl disable"}
	found := false
	for _, sig := range suspicious {
		if strings.Contains(strContent, sig) {
			fmt.Println(NewMessage(chalk.Red, "Suspicious content '"+sig+"' found in "+path))
			found = true
		}
	}

	if found && autoRemove {
		fmt.Println(NewMessage(chalk.Yellow, "Backing up and cleaning "+path+"..."))
		backupPath := path + ".defend_bak"
		os.Rename(path, backupPath)
		cleanFile(backupPath, path, suspicious)
	}
}

func cleanFile(source, dest string, badStrings []string) {
	input, err := os.ReadFile(source)
	if err != nil {
		return
	}

	lines := strings.Split(string(input), "\n")
	output := []string{}

	for _, line := range lines {
		isBad := false
		for _, bad := range badStrings {
			if strings.Contains(line, bad) {
				isBad = true
				break
			}
		}
		if !isBad {
			output = append(output, line)
		}
	}

	os.WriteFile(dest, []byte(strings.Join(output, "\n")), 0644)
}
