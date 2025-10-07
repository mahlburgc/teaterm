package internal

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	CmdHistoryLines []string
	CmdHistFile     string
}

// Setup / load the teaterm configuration.
// Currently config only contains the command history.
func GetConfig() Config {
	var config Config

	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	cmdHist := homedir + "/.config/teaterm/"
	cmdHistFileName := "cmdhistroy.conf"

	config.CmdHistFile = cmdHist + cmdHistFileName

	err = os.MkdirAll(cmdHist, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	cmdhist, err := os.ReadFile(config.CmdHistFile)
	if err != nil {
		if os.IsNotExist(err) {
			cmdhist = nil
		} else {
			log.Fatal(err)
		}
	}

	if cmdhist != nil {
		// Trim leading/trailing whitespace
		trimmedCmdHist := strings.TrimSpace(string(cmdhist))
		// Split the string by the newline character
		config.CmdHistoryLines = strings.Split(trimmedCmdHist, "\n")
	}
	return config
}
