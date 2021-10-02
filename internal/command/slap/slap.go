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
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/jmcvetta/randutil"
)

// CommandData returns request
func CommandData() api.CreateCommandData {
	return api.CreateCommandData{
		Name:        "slap",
		Description: "slap someone with a trout",
		Options: []discord.CommandOption{
			{
				Name:        "target",
				Type:        discord.UserOption,
				Description: "who to slap, a mention",
				Required:    true,
			},
			{
				Name: "reason",
				Type: discord.StringOption,
				Description: "optional addendum to the slap output, appended after `slaps <x> " +
					"with a trout <addendum>",
				Required: false,
			},
		},
	}
}

// HandleSlap generates and sends a slap-sentence for the author and target of the given message
func HandleSlap(author discord.User, targetID string, reason string) (
	response *api.InteractionResponseData, err error,
) {
	det, adj, err := getAdjective()
	if err != nil {
		err = fmt.Errorf("error occurred getting adjective: %v", err)
		return
	}

	targetSnowflake, err := discord.ParseSnowflake(targetID)
	if err != nil {
		err = fmt.Errorf("error parsing target ID: %v", err)
		return
	}

	response = &api.InteractionResponseData{
		Content: option.NewNullableString(fmt.Sprintf(
			"%s slaps %s around with %s %s trout %s",
			author.Username, discord.UserID(targetSnowflake).Mention(), det, adj, reason),
		),
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
