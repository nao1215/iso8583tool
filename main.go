package main

import (
	"os"

	"github.com/nao1215/iso8583tool/cmd"
)

func main() {
	workDir, err := os.Getwd()
	if err != nil {
		// When stderr is gone too, the exit code is all that is left to report.
		if _, werr := os.Stderr.WriteString(err.Error() + "\n"); werr != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}

	app := cmd.NewApp(os.Stdout, os.Stderr, os.Stdin, workDir)
	os.Exit(app.Run(os.Args[1:]))
}
