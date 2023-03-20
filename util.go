package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	progressbar "github.com/schollz/progressbar/v3"
)

func showSpinner(bar *progressbar.ProgressBar, done chan bool) {
	tick := time.Tick(100 * time.Millisecond)
	for {
		select {
		case <-tick:
			bar.Add(1)
		case <-done:
			return
		}
	}
}

func getProgressBar() *progressbar.ProgressBar {
	return progressbar.NewOptions64(
		-1,
		progressbar.OptionSetDescription("Waiting for ChatGPT to respond..."),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionSetWidth(10),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
	)
}

func executeScript(script string) error {
	cmd := exec.Command("bash", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
