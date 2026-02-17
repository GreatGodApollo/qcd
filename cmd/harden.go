package cmd

import (
	"bufio"
	_ "embed" // Use blank identifier for embed
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ttacon/chalk"
)

//go:embed embedded/audit.rules
var auditRules string

//go:embed embedded/fallback.nft
var fallbackNft string

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

		fmt.Println(NewMessage(chalk.Blue, "Setting up Auditd Rules..."))
		if err := setupAuditd(); err != nil {
			fmt.Println(NewMessage(chalk.Red, "Error setting up auditd: "+err.Error()))
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

	// 2. Fallback to embedded nft rules
	fallbackPath := "/tmp/fallback.nft"
	if err := os.WriteFile(fallbackPath, []byte(fallbackNft), 0600); err != nil {
		return fmt.Errorf("failed to write fallback rules: %w", err)
	}

	if err := RunCommand("nft", "-f", fallbackPath); err != nil {
		return fmt.Errorf("failed to apply fallback rules: %w", err)
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

func setupAuditd() error {
	if len(auditRules) == 0 {
		return fmt.Errorf("embedded audit rules are empty")
	}

	// Determine location: Fedora uses /etc/audit/rules.d/ usually
	targetPath := "/etc/audit/rules.d/qcd.rules"
	if _, err := os.Stat("/etc/audit/rules.d"); os.IsNotExist(err) {
		// Fallback to direct file if directory doesn't exist
		targetPath = "/etc/audit/audit.rules"
	}

	fmt.Println(NewMessage(chalk.Yellow, "Writing audit rules to "+targetPath))

	if err := os.WriteFile(targetPath, []byte(auditRules), 0600); err != nil {
		return err
	}

	// Reload logic
	fmt.Println(NewMessage(chalk.Yellow, "Reloading auditd..."))
	// Try augenrules first if in rules.d
	if strings.Contains(targetPath, "rules.d") {
		if err := RunCommand("augenrules", "--load"); err != nil {
			fmt.Println(NewMessage(chalk.Red, "augenrules failed, trying service reload..."))
			RunCommand("service", "auditd", "restart")
		}
	} else {
		RunCommand("service", "auditd", "restart")
	}

	return nil
}
