package command

import (
	"errors"
	"strings"
)

var (
	errNotEnoughArgs = errors.New("not enough arguments")
)

// GetCommand parses a message string and returns it as parts of a command
func GetCommand(message string) (string, string, string, error) {
	index := strings.Index(message, "!")
	if index == -1 {
		return "", "", "", errors.New("no scheme found")
	}
	command := message[index+1:]
	if len(command) == 0 || command[0] == ' ' {
		return "", "", "", errors.New("invalid command")
	}

	splits := strings.Split(command, " ")

	if len(splits) < 1 {
		return "", "", "", errNotEnoughArgs
	}
	source := splits[0]

	var scheme string
	if len(splits) > 1 {
		scheme = splits[1]
	}

	var argument string
	if len(splits) >= 3 {
		argument = strings.Join(splits[2:], " ")
	}
	return source, scheme, argument, nil
}
