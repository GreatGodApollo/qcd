package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ttacon/chalk"
	"os"
	"strings"
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
		if createFileIfNotExist(home+"/", ".qcd.toml") != nil {
			os.Exit(1)
		}
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println(NewMessage(chalk.Magenta, "Using config file:").ThenColor(chalk.Green, viper.ConfigFileUsed()))
	}
	// Preload configurations
	if CheckError(viper.WriteConfig()) {
		os.Exit(1)
	}
}

func createFileIfNotExist(dir, file string) error {
	if _, err := os.Stat(dir + file); err != nil {
		if os.IsNotExist(err) {
			f, err := os.Create(dir + file)
			if CheckError(err) {
				return err
			}
			err = f.Close()
			if CheckError(err) {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}
