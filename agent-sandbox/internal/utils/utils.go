package utils

import (
	"os"
)

const CommitMessageFile = "commit-message.txt"

func GetContentFile(file string) (string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}

	return string(content), nil
}
