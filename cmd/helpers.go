package cmd

import (
	"fmt"
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
