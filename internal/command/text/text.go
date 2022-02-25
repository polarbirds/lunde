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
					},
				},
				{
					Name:        "message",
					Type:        discord.StringOption,
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
	event *gateway.InteractionCreateEvent,
	options map[string]string,
) (
	response *api.InteractionResponseData, err error,
) {
	msg := options["message"]
	if msg == "" {
		if lastMsg, ok := th.srv.LastMessages[event.ChannelID]; ok {
			msg = lastMsg.Content
		} else {
			err = errors.New("found no message from options or in channel")
			return
		}
	}

	return &api.InteractionResponseData{
		Content: option.NewNullableString(convert(msg, options["algo"])),
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
