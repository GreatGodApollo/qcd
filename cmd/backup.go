package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ttacon/chalk"
)

var installRestic bool

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup critical services (Dovecot/Postfix)",
	Long:  `Backs up Dovecot and Postfix configurations. Can automatically download and configure restic and resticprofile for robust backups.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(NewMessage(chalk.Green, "Starting Backup Process..."))

		// 1. Basic Tarball Backup of Configs
		backupConfigs()

		// 2. Restic Integration
		if installRestic {
			if err := setupRestic(); err != nil {
				fmt.Println(NewMessage(chalk.Red, "Restic Setup Failed: "+err.Error()))
			} else {
				fmt.Println(NewMessage(chalk.Green, "Restic Setup Complete. Running Backup Profile..."))
				RunCommand("resticprofile", "backup", "system")
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(backupCmd)
	backupCmd.Flags().BoolVarP(&installRestic, "restic", "r", false, "Install and configure restic/resticprofile")
}

func backupConfigs() {
	dest := "./backups"
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		os.Mkdir(dest, 0755)
	}

	targets := viper.GetStringSlice("backup.targets")
	if len(targets) == 0 {
		// Fallback if config is missing or empty
		targets = []string{"/etc/dovecot", "/etc/postfix"}
	}

	timestamp := "backup" // In real app, add timestamp. using fixed name for simplicity/test
	tarName := filepath.Join(dest, fmt.Sprintf("mail_config_%s.tar.gz", timestamp))

	fmt.Println(NewMessage(chalk.Blue, "Creating tarball of mail configs..."))
	// tar -czf ...
	args := []string{"-czf", tarName}
	args = append(args, targets...)

	if err := RunCommand("tar", args...); err != nil {
		fmt.Println(NewMessage(chalk.Red, "Failed to create backup tarball: "+err.Error()))
	} else {
		fmt.Println(NewMessage(chalk.Green, "Backup created at "+tarName))
	}
}

func setupRestic() error {
	// Auto-download restic and resticprofile
	// Assuming Linux amd64 for CCDC usually
	arch := runtime.GOARCH
	if arch != "amd64" {
		return fmt.Errorf("auto-download currently only supports amd64, got %s", arch)
	}

	// URLs (hardcoded for now, should be dynamic in prod)
	resticUrl := "https://github.com/restic/restic/releases/download/v0.16.4/restic_0.16.4_linux_amd64.bz2"
	resticProfileUrl := "https://github.com/creativeprojects/resticprofile/releases/download/v0.26.0/resticprofile_0.26.0_linux_amd64.tar.gz"

	binDir := "/usr/local/bin"

	// Check if installed
	if err := RunCommand("restic", "version"); err != nil {
		fmt.Println(NewMessage(chalk.Yellow, "Restic not found. Downloading..."))
		if err := DownloadFile("/tmp/restic.bz2", resticUrl); err != nil {
			return err
		}
		RunCommand("bzip2", "-d", "/tmp/restic.bz2")
		RunCommand("install", "-m", "0755", "/tmp/restic", filepath.Join(binDir, "restic"))
	}

	if err := RunCommand("resticprofile", "version"); err != nil {
		fmt.Println(NewMessage(chalk.Yellow, "ResticProfile not found. Downloading..."))
		if err := DownloadFile("/tmp/resticprofile.tar.gz", resticProfileUrl); err != nil {
			return err
		}
		RunCommand("tar", "-xzf", "/tmp/resticprofile.tar.gz", "-C", "/tmp")
		RunCommand("install", "-m", "0755", "/tmp/resticprofile", filepath.Join(binDir, "resticprofile"))
	}

	// Configure Profile
	configPath := "profiles.yaml" // current dir or /etc/resticprofile/profiles.yaml
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println(NewMessage(chalk.Yellow, "Creating basic resticprofile config..."))
		configContent := `
system:
  repository: "local:/var/backups/restic"
  password: "changeme_ccdc_password"
  initialize: true
  backup:
    source:
      - "/etc/dovecot"
      - "/etc/postfix"
      - "/var/www"
    schedule: "Every 15 minutes"
    retention:
      keep-last: 5
`
		// Note: Password should be prompted or secure. Using hardcoded for CCDC quick start.
		os.WriteFile(configPath, []byte(configContent), 0600)
	}

	return nil
}
