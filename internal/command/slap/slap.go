package slap

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/jmcvetta/randutil"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/polarbirds/lunde/internal/server"
)

// CreateCommand creates a lunde command to slap people
func CreateCommand(_ *server.Server) (cmd command.LundeCommand, err error) {
	cmd = command.LundeCommand{
		HandleInteraction: handleInteraction,
		CommandData: api.CreateCommandData{
			Name:        "slap",
			Description: "slap someone with a trout",
			Options: []discord.CommandOption{
				&discord.UserOption{
					OptionName:  "target",
					Description: "who to slap",
					Required:    true,
				},
				&discord.StringOption{
					OptionName: "reason",
					Description: "optional addendum to the slap output, appended after `slaps " +
						"<x> with a trout <addendum>",
					Required: false,
				},
			},
		},
	}

	return
}

func handleInteraction(event *gateway.InteractionCreateEvent, options map[string]string) (
	response *api.InteractionResponseData, err error,
) {
	det, adj, err := getAdjective()
	if err != nil {
		err = fmt.Errorf("getting adjective: %v", err)
		return
	}

	targetSnowflake, err := discord.ParseSnowflake(options["target"])
	if err != nil {
		err = fmt.Errorf("parsing target ID: %v", err)
		return
	}

	response = &api.InteractionResponseData{
		Content: option.NewNullableString(
			fmt.Sprintf(
				"%s slaps %s around with %s %s trout %s",
				event.Member.User.Username,
				discord.UserID(targetSnowflake).Mention(),
				det,
				adj,
				options["reason"])),
	}

	return
}

type wordResponse struct {
	Word  string `json:"word"`
	Score int    `json:"score"`
}

type adjectiveResponse []wordResponse

func getAdjective() (determiner string, adjective string, err error) {
	var res *http.Response
	res, err = http.Get("https://describingwords.io/api/descriptors?term=fish&sortType=frequency")
	if err != nil {
		err = fmt.Errorf("error contacting adjective server: %v", err)
		return
	}

	defer res.Body.Close()

	bodBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		err = fmt.Errorf(
			"failed reading response body from adjective-server (status was %s): %v",
			res.Status, err,
		)
		return
	}

	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf(
			"adjective-server returned status %s and body: %s",
			res.Status, string(bodBytes),
		)
		return
	}

	bod := adjectiveResponse{}
	err = json.Unmarshal(bodBytes, &bod)
	if err != nil {
		err = fmt.Errorf("error unmarshalling adjective response: %v", err)
		return
	}

	for i, choice := range bod {
		choice.Score = int(math.Sqrt(float64(choice.Score)))
		bod[i] = choice
	}

	adjective, err = chooseAdjective(bod)
	if err != nil {
		err = fmt.Errorf("error choosing adjective: %v", err)
		return
	}

	determiner = chooseDeterminer(adjective)
	return
}

func chooseAdjective(adjectives adjectiveResponse) (adjective string, err error) {
	choices := make([]randutil.Choice, 0, 2)
	for _, adj := range adjectives {
		choices = append(choices, randutil.Choice{
			Weight: adj.Score,
			Item:   adj.Word,
		})
	}

	chosen, err := randutil.WeightedChoice(choices)
	if err != nil {
		return
	}

	adjective = chosen.Item.(string)
	return
}

func chooseDeterminer(adjective string) string {
	var firstPart string
	if len(adjective) == 0 {
		firstPart = "erroneous"
	} else {
		firstPart = strings.Split(adjective, " ")[0]
	}
	if strings.HasSuffix(firstPart, "est") {
		return "the"
	}

	switch firstPart[0] {
	case 'a', 'e', 'i', 'o', 'u':
		return "an"
	default:
		return "a"
	}
}
