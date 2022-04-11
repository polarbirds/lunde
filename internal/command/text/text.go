package text

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"unicode"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/kortschak/zalgo"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/polarbirds/lunde/internal/server"
)

type textHandler struct {
	srv *server.Server
}

// CreateCommand creates a lunde command handling text-conversion
func CreateCommand(srv *server.Server) (cmd command.LundeCommand, err error) {
	th := textHandler{srv}
	cmd = command.LundeCommand{
		HandleInteraction: th.handleInteraction,
		CommandData: api.CreateCommandData{
			Name:        "text",
			Description: "corrupt or convert a given message, or the last message in the channel",
			Options: []discord.CommandOption{
				&discord.StringOption{
					OptionName:  "algo",
					Description: "what algorithm to convert/corrupt the text with",
					Required:    true,
					Choices: []discord.StringChoice{
						{Name: "spunge", Value: "spunge"},
						{Name: "zalgo", Value: "zalgo"},
						{Name: "chonk", Value: "chonk"},
					},
				},
				&discord.StringOption{
					OptionName:  "message",
					Description: "optional message to convert/corrupt",
					Required:    false,
				},
			},
		},
	}

	return
}

// HandleText handles the text-command, converting/corrupting the given message in a way decided by
// the value of algo
func (th *textHandler) handleInteraction(
	event *gateway.InteractionCreateEvent, options map[string]discord.CommandInteractionOption,
) (
	response *api.InteractionResponseData, err error,
) {
	msg := options["message"].String()
	if msg == "" {
		lastMsg, messageFound := th.srv.LastMessages[event.ChannelID]
		if !messageFound {
			err = errors.New("found no message from options or in channel")
			return
		}

		msg = lastMsg.Content
	}

	return &api.InteractionResponseData{
		Content: option.NewNullableString(convert(msg, options["algo"].String())),
	}, nil
}

func convert(content string, algo string) string {
	switch algo {
	case "zalgo":
		return zalgoPlz(content)
	case "spunge":
		return spungePlz(content)
	case "chonk":
		return chonkPlz(content)
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

	fmt.Fprint(z, content)
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

func chonkPlz(content string) string {
	contentRunes := []rune(content)
	for i, r := range contentRunes {
		// if rune is ~alpha
		if r >= 0x0021 && r <= 0x007E {
			contentRunes[i] = r - 0x0041 + 0xFF21
		}
	}
	return string(contentRunes)
}
