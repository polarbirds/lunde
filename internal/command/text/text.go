package text

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"unicode"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/kortschak/zalgo"
)

// CommandData returns request
func CommandData() api.CreateCommandData {
	return api.CreateCommandData{
		Name:        "text",
		Description: "corrupt or converts a given message or the last message",
		Options: []discord.CommandOption{
			{
				Name:        "algo",
				Type:        discord.StringOption,
				Description: "what algorithm to convert/corrupt the text with",
				Required:    true,
				Choices: []discord.CommandOptionChoice{
					{Name: "spunge", Value: "spunge"},
					{Name: "zalgo", Value: "zalgo"},
					{Name: "chirese", Value: "chirese"},
				},
			},
			{
				Name:        "message",
				Type:        discord.StringOption,
				Description: "optional message to convert/corrupt",
				Required:    false,
			},
		},
	}
}

// HandleText handles the text-command, converting/corrupting the given message in a way decided by
// the value of algo
func HandleText(
	algo string,
	message string,
) (*api.InteractionResponseData, error) {
	if message == "" {
		return nil, errors.New("empty message")
	}

	return &api.InteractionResponseData{
		Content: option.NewNullableString(convert(message, algo)),
	}, nil
}

func convert(content string, algo string) string {
	switch algo {
	case "zalgo":
		return zalgoPlz(content)
	case "spunge":
		return spungePlz(content)
	default:
		return content
	}
}

func zalgoPlz(content string) string {
	w := bytes.NewBufferString("")
	z := zalgo.NewCorrupter(w)

	z.Zalgo = func(n int, r rune, z *zalgo.Corrupter) bool {
		z.Up += 0.1
		z.Middle += complex(0.01, 0.01)
		z.Down += complex(real(z.Down)*0.1, 0)
		return false
	}

	z.Up = complex(0, 0.2)
	z.Middle = complex(0, 0.2)
	z.Down = complex(0.001, 0.3)

	fmt.Fprintf(z, content)
	return w.String()
}

func spungePlz(content string) string {
	contentRunes := []rune(strings.ToLower(content))
	lastCharConverted := false
	for i, c := range contentRunes {
		setSize := 2
		if lastCharConverted {
			setSize = 3
		}
		if rand.Intn(setSize) == 0 {
			contentRunes[i] = unicode.ToUpper(c)
			lastCharConverted = true
		} else {
			lastCharConverted = false
		}
	}
	return string(contentRunes)
}

var chiresePattern = regexp.MustCompile("asd")

const tmp = "||tmp||"

func swap(src string, c1 string, c2 string) string {
	src = strings.Replace(src, c1, tmp, -1) // c1 -> tmp
	src = strings.Replace(src, c2, c1, -1)  // c2 -> c1
	src = strings.Replace(src, tmp, c2, -1) // tmp -> c2

	return src
}
