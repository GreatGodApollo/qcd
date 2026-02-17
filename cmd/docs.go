package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/ttacon/chalk"
	"log"
	"os"
)

var dir string

// docsCmd represents the docs command
var docsCmd = &cobra.Command{
	Use:   "docs (md|man|rst)",
	Short: "Documentation Generator",
	Long: `A super quick command to generate documentation for Quick CCDC Defender
You can provide a single argument: "md", "man", or "rst" to specify what format you'd like the docs in.
Each of those being MarkDown, Man Pages, or ReStructured Text respectively.`,
	ValidArgs: []string{"md", "man", "rst"},
	Args:      cobra.OnlyValidArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err := os.Mkdir(dir, os.ModeDir)
			if err != nil {
				log.Fatal(err)
			}
		}
		if len(args) > 0 {
			switch args[0] {
			case "md":
				{
					err := doc.GenMarkdownTree(rootCmd, dir)
					if err != nil {
						log.Fatal(err)
					}
					break
				}
			case "man":
				{
					header := &doc.GenManHeader{
						Title:   "QSR",
						Section: "1",
					}
					err := doc.GenManTree(rootCmd, header, dir)
					if err != nil {
						log.Fatal(err)
					}
					break
				}
			case "rst":
				{
					err := doc.GenReSTTree(rootCmd, dir)
					if err != nil {
						log.Fatal(err)
					}
					break
				}
			}
		} else {
			err := doc.GenMarkdownTree(rootCmd, dir)
			if CheckError(err) {
				return
			}
		}
		fmt.Println(NewMessage(chalk.Green, "Documentation has been generated in: ").
			ThenColor(chalk.Cyan, dir))
	},
}

func init() {
	rootCmd.AddCommand(docsCmd)

	docsCmd.Flags().StringVarP(&dir, "directory", "d", "./", "Directory to create documentation in.")
}
