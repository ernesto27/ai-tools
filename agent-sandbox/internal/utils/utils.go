package utils

import (
	"os"
)

func GetContentFile(file string) (string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}

	return string(content), nil
}
