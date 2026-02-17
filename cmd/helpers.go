package cmd

import (
	"fmt"
	"github.com/go-cmd/cmd"
	"github.com/ttacon/chalk"
	"io"
	"net/http"
	"os"
)

func CheckError(err error) bool {
	if err != nil {
		fmt.Println(NewMessage(chalk.Red, err.Error()).String())
		return true
	}
	return false
}

func DownloadFile(filepath string, url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func RunCommand(executable string, args ...string) error {
	// Create Cmd with options
	cmO := cmd.Options{
		Buffered:  false,
		Streaming: true,
	}

	envCmd := cmd.NewCmdOptions(cmO, executable, args...)

	// Print STDOUT and STDERR lines streaming from Cmd
	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)
		for envCmd.Stdout != nil || envCmd.Stderr != nil {
			select {
			case line, open := <-envCmd.Stdout:
				if !open {
					envCmd.Stdout = nil
					continue
				}
				fmt.Println(line)
			case line, open := <-envCmd.Stderr:
				if !open {
					envCmd.Stderr = nil
					continue
				}
				fmt.Fprintln(os.Stderr, line)
			}
		}
	}()

	// Run and wait for Cmd to return, discard Status
	stat := <-envCmd.Start()
	// Wait for goroutine to print everything
	<-doneChan
	return stat.Error
}
