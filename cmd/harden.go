package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ttacon/chalk"
)

var firewallOnly bool

var hardenCmd = &cobra.Command{
	Use:   "harden",
	Short: "Harden the system and apply firewall rules",
	Long:  `Applies various hardening measures including firewall rules, locking down cron/at, and enforcing nologin shells.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(NewMessage(chalk.Green, "Starting System Hardening..."))

		// Firewall Logic
		fmt.Println(NewMessage(chalk.Blue, "Applying Firewall Rules..."))
		if err := applyFirewall(); err != nil {
			fmt.Println(NewMessage(chalk.Red, "Error applying firewall: "+err.Error()))
		} else {
			fmt.Println(NewMessage(chalk.Green, "Firewall Applied Successfully!"))
		}

		if firewallOnly {
			return
		}

		// Hardening Logic
		fmt.Println(NewMessage(chalk.Blue, "Locking down Cron and At..."))
		if err := lockdownCronAt(); err != nil {
			fmt.Println(NewMessage(chalk.Red, "Error locking down cron/at: "+err.Error()))
		}

		fmt.Println(NewMessage(chalk.Blue, "Enforcing Nologin Shells..."))
		if err := enforceNologin(); err != nil {
			fmt.Println(NewMessage(chalk.Red, "Error enforcing nologin: "+err.Error()))
		}
	},
}

func init() {
	rootCmd.AddCommand(hardenCmd)
	hardenCmd.Flags().BoolVarP(&firewallOnly, "firewall-only", "f", false, "Only run firewall logic")
}

func applyFirewall() error {
	// 1. Attempt to download nftbuild
	nftBuildUrl := "https://github.com/UWStout-CCDC/CCDC-scripts/raw/refs/heads/master/firewall/hostfirewall/nftbuild"
	nftBuildPath := "/tmp/nftbuild"

	fmt.Println(NewMessage(chalk.Yellow, "Attempting to download nftbuild script..."))
	err := DownloadFile(nftBuildPath, nftBuildUrl)
	if err == nil {
		// Download successful
		err = os.Chmod(nftBuildPath, 0755)
		if err == nil {
			fmt.Println(NewMessage(chalk.Green, "nftbuild downloaded. Executing..."))
			err = RunCommand(nftBuildPath, "-sys", "mail")
			if err == nil {
				return nil
			}
			fmt.Println(NewMessage(chalk.Red, "nftbuild execution failed: "+err.Error()))
		} else {
			fmt.Println(NewMessage(chalk.Red, "Failed to chmod nftbuild: "+err.Error()))
		}
	} else {
		fmt.Println(NewMessage(chalk.Red, "Failed to download nftbuild: "+err.Error()))
	}

	fmt.Println(NewMessage(chalk.Yellow, "Falling back to internal nftables rules..."))

	// 2. Fallback to nft direct commands
	// basic mail server rules
	commands := [][]string{
		{"nft", "flush", "ruleset"},
		{"nft", "add", "table", "inet", "filter"},
		{"nft", "add", "chain", "inet", "filter", "input", "{ type filter hook input priority 0; policy drop; }"},
		{"nft", "add", "chain", "inet", "filter", "forward", "{ type filter hook forward priority 0; policy drop; }"},
		{"nft", "add", "chain", "inet", "filter", "output", "{ type filter hook output priority 0; policy accept; }"},
		{"nft", "add", "rule", "inet", "filter", "input", "iif", "lo", "accept"},
		{"nft", "add", "rule", "inet", "filter", "input", "ct", "state", "established,related", "accept"},
		// SSH
		{"nft", "add", "rule", "inet", "filter", "input", "tcp", "dport", "22", "accept"},
		// SMTP (25, 465, 587)
		{"nft", "add", "rule", "inet", "filter", "input", "tcp", "dport", "{ 25, 465, 587 }", "accept"},
		// IMAP (143, 993)
		{"nft", "add", "rule", "inet", "filter", "input", "tcp", "dport", "{ 143, 993 }", "accept"},
		// POP3 (110, 995)
		{"nft", "add", "rule", "inet", "filter", "input", "tcp", "dport", "{ 110, 995 }", "accept"},
		// ICMP
		{"nft", "add", "rule", "inet", "filter", "input", "ip", "protocol", "icmp", "accept"},
	}

	for _, cmd := range commands {
		if err := RunCommand(cmd[0], cmd[1:]...); err != nil {
			return fmt.Errorf("failed to run command %v: %w", cmd, err)
		}
	}

	return nil
}

func lockdownCronAt() error {
	files := []string{"/etc/cron.deny", "/etc/at.deny"}
	for _, file := range files {
		if err := os.WriteFile(file, []byte("ALL\n"), 0644); err != nil {
			fmt.Println(NewMessage(chalk.Red, "Failed to write to "+file+": "+err.Error()))
			// Don't error out completely, try the next one
		} else {
			fmt.Println(NewMessage(chalk.Green, "Wrote ALL to "+file))
		}
	}
	return nil
}

func enforceNologin() error {
	whitelist := viper.GetStringSlice("harden.shell_whitelist")
	// convert to map for O(1) lookup
	whitelisted := make(map[string]bool)
	for _, u := range whitelist {
		whitelisted[u] = true
	}

	file, err := os.Open("/etc/passwd")
	if err != nil {
		return err
	}
	defer file.Close()

	var usersToLock []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) > 6 {
			user := parts[0]
			shell := parts[6]

			// Skip if already nologin or false
			if strings.Contains(shell, "nologin") || strings.Contains(shell, "false") {
				continue
			}

			// Skip if whitelisted
			if whitelisted[user] {
				continue
			}

			usersToLock = append(usersToLock, user)
		}
	}

	if len(usersToLock) > 0 {
		fmt.Println(NewMessage(chalk.Yellow, fmt.Sprintf("Found %d users to lock:", len(usersToLock))))
		for _, u := range usersToLock {
			fmt.Println(" - " + u)
			// Execute usermod
			err := RunCommand("usermod", "-s", "/sbin/nologin", u)
			if err != nil {
				fmt.Println(NewMessage(chalk.Red, "Failed to lock "+u+": "+err.Error()))
			} else {
				fmt.Println(NewMessage(chalk.Green, "Locked "+u))
			}
		}
	} else {
		fmt.Println(NewMessage(chalk.Green, "No users found needing nologin enforcement (based on whitelist)."))
	}
	return nil
}
