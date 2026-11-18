package internal

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type Config struct {
	CmdHistoryLines []string
	Settings        Settings
}

// Return the path to the config file.
// If the path does not exist, it will be created.
func getConfigFilePath() string {
	cmdHistFileName := "cmdhistroy.conf"
	return configDir() + cmdHistFileName
}

// Return the teaterm config directory (with trailing slash).
// If the directory does not exist, it will be created.
func configDir() string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	configBasePath := homedir + "/.config/teaterm/"

	err = os.MkdirAll(configBasePath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	return configBasePath
}

// Setup / load the teaterm configuration: the command history and the
// user settings (teaterm.toml). Exits on an invalid settings file.
func GetConfig() Config {
	var config Config

	settings, err := LoadSettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "teaterm: %v\n", err)
		os.Exit(1)
	}
	config.Settings = settings

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
	const maxStoredCmds = 500
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
