package slap

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jmcvetta/randutil"
)

// Generate generates and sends a slap-sentence for the author and target of the given message
func Generate(target string, reason string, s *discordgo.Session, m *discordgo.MessageCreate) (
	reply *discordgo.Message, err error, discErr error,
) {
	det, adj, err := getAdjective()
	if err != nil {
		err = fmt.Errorf("error occurred getting adjective: %v", err)
		return
	}

	var user *discordgo.User
	if len(target) < 5 {
		reply, discErr = s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"%s slap themselves around with a short target and %s %s trout",
				m.Author.Username, det, adj,
			),
		)
		err = fmt.Errorf("too short target string: %s", target)
		return
	}

	var startIndex int
	if strings.HasPrefix(target, "<@!") {
		startIndex = 3
	} else if strings.HasPrefix(target, "<@") {
		startIndex = 2
	} else {
		reply, discErr = s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"%s slap themselves around with a target-string that does not look like a mention "+
					"and %s %s trout",
				m.Author.Username, det, adj,
			),
		)
		err = fmt.Errorf("invalid target string: %s", target)
		return
	}

	user, err = s.User(target[startIndex : len(target)-1])
	if err != nil {
		reply, discErr = s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"%s slap themselves around with a target-string that failed resolving "+
					"and %s %s trout",
				m.Author.Username, det, adj,
			),
		)
		err = fmt.Errorf("error getting user when using target string: %v", err)
		return
	}

	if user.Bot {
		if reason != "" {
			reason = "because they are trying to be smart"
		}
		reply, discErr = s.ChannelMessageSend(
			m.ChannelID,
			fmt.Sprintf(
				"%s makes %s slap themselves around with %s %s trout %s",
				user.Username, m.Author.Mention(), det, adj, reason,
			),
		)
		return
	}

	reply, discErr = s.ChannelMessageSend(
		m.ChannelID,
		fmt.Sprintf(
			"%s slaps %s around with %s %s trout %s",
			m.Author.Username, user.Mention(), det, adj, reason,
		),
	)

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
