package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ttacon/chalk"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:     "qcd <command> [<arguments>]",
	Short:   "A quick and easy way to defend during CCDC",
	Long:    "Quick CCDC Defender is a command line utility that allows you to quickly defend your system.",
	Version: "0.0.1",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if strings.HasPrefix(err.Error(), "unknown command") {
			return
		}
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.qcd.toml)")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if CheckError(err) {
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.SetConfigName(".qcd")
		viper.SetConfigType("toml")

		// Set Defaults
		viper.SetDefault("backup.targets", []string{"/etc/dovecot", "/etc/postfix"})
		viper.SetDefault("backup.dest", "./backups")
		viper.SetDefault("harden.shell_whitelist", []string{"root", "sysadmin"})
		viper.SetDefault("persistence.ignore_users", []string{"root", "sysadmin"})

		if err := createFileIfNotExist(home+"/", ".qcd.toml"); err != nil {
			os.Exit(1)
		}
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println(NewMessage(chalk.Magenta, "Using config file:").ThenColor(chalk.Green, viper.ConfigFileUsed()))
	}
	// If read fails (e.g. empty file just created), write defaults
	if err := viper.SafeWriteConfig(); err != nil {
		// If SafeWrite fails (already exists), try WriteConfig
		viper.WriteConfig()
	}
}

func createFileIfNotExist(dir, file string) error {
	path := filepath.Join(dir, file)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			// rely on viper to write the file with defaults, but we need to ensure directory exists?
			// Actually viper.SafeWriteConfig() can create the file.
			// But strict requirement: "create a config file on the first run".
			// standard viper flow:
			// 1. Set Defaults
			// 2. ReadConfig (fails if not exist)
			// 3. If failed, SafeWriteConfig (creates file with defaults)
			return nil
		}
		return err
	}
	return nil
}
