package define

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/polarbirds/lunde/internal/server"
)

type udResponse struct {
	List []struct {
		Word       string `json:"word"`
		Definition string `json:"definition"`
		Example    string `json:"example"`
		ThumbsUp   int    `json:"thumbs_up"`
		ThumbsDown int    `json:"thumbs_down"`
	} `json:"list"`
}

// CreateCommand creates the define command
func CreateCommand(_ *server.Server) (cmd command.LundeCommand, err error) {
	cmd = command.LundeCommand{
		HandleInteraction: handleInteraction,
		CommandData: api.CreateCommandData{
			Name: "define",
			Description: "fetch a definition of a word of phrase from a reputable and " +
				"renowned source of knowledge",
			Options: []discord.CommandOption{
				&discord.StringOption{
					OptionName:  "term",
					Description: "term to fetch definition for",
					Required:    true,
				},
			},
		},
	}
	return
}

func handleInteraction(
	_ *gateway.InteractionCreateEvent,
	options map[string]discord.CommandInteractionOption,
) (
	response *api.InteractionResponseData, err error,
) {
	term := options["term"]
	var res *http.Response
	res, err = http.Get("http://api.urbandictionary.com/v0/define?term=" +
		url.QueryEscape(term.String()))
	if err != nil {
		err = fmt.Errorf("error contacting define server: %v", err)
		return
	}

	defer res.Body.Close()

	bodBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		err = fmt.Errorf(
			"failed reading response body from define-server (status was %s): %v",
			res.Status, err,
		)
		return
	}

	var udRes udResponse
	err = json.Unmarshal(bodBytes, &udRes)
	if err != nil {
		err = errors.New("no definitions found")
		return
	}

	if len(udRes.List) == 0 {
		err = fmt.Errorf("no definition returned for the term: %s", term)
		return
	}

	replyContent := fmt.Sprintf("Definition(s) for **%s**:", term)
	for i := 0; i < 3 && i < len(udRes.List); i++ {
		def := udRes.List[i]
		replyContent += fmt.Sprintf("\n%d) **%s**: %s ⬆%v / ⬇%v\n%s\n",
			i+1, def.Word, sanitizeUrbanDictionaryText(def.Definition),
			def.ThumbsUp, def.ThumbsDown,
			sanitizeUrbanDictionaryText(def.Example))
	}

	response = &api.InteractionResponseData{
		Content: option.NewNullableString(replyContent),
	}

	return
}

func sanitizeUrbanDictionaryText(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "]", ""), "[", "")
}
