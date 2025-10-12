package internal

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	CmdHistoryLines []string
}

// Return the path to the config file.
// If the path does not exist, it will be created.
func getConfigFilePath() string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	cmdHistBasePath := homedir + "/.config/teaterm/"
	cmdHistFileName := "cmdhistroy.conf"

	err = os.MkdirAll(cmdHistBasePath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	return cmdHistBasePath + cmdHistFileName
}

// Setup / load the teaterm configuration.
// Currently config only contains the command history.
func GetConfig() Config {
	var config Config
	filepath := getConfigFilePath()

	cmdHist, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			cmdHist = nil
		} else {
			log.Fatal(err)
		}
	}

	if cmdHist != nil {
		// Trim leading/trailing whitespace
		trimmedCmdHist := strings.TrimSpace(string(cmdHist))
		// Split the string by the newline character
		config.CmdHistoryLines = strings.Split(trimmedCmdHist, "\n")
	}
	return config
}

// Store the current config.
// Saves the last x elemets of current command history for next session.
func StoreConfig(cmdHist []string) {
	const maxStoredCmds = 100
	if len(cmdHist) > maxStoredCmds {
		start := len(cmdHist) - maxStoredCmds
		cmdHist = cmdHist[start:]
	}

	filepath := getConfigFilePath()

	fileContent := strings.Join(cmdHist, "\n") + "\n"

	err := os.WriteFile(filepath, []byte(fileContent), 0o644)
	if err != nil {
		log.Fatal(err)
	}
}
