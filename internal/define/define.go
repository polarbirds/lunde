package define

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// Define fetches and sends a definition for the given term
func Define(term string, s *discordgo.Session, m *discordgo.MessageCreate) (
	reply *discordgo.Message, err error, discErr error,
) {
	if term == "" {
		err = errors.New("term is empty boi, use !define termShouldBeHereDumbass")
		return
	}
	var res *http.Response
	res, err = http.Get("https://describingwords.io/api/define?term=" + term)
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

	bodString := string(bodBytes)
	if bodString == "" {
		err = fmt.Errorf("no definition returned for the term: %s", term)
		return
	}

	reply, discErr = s.ChannelMessageSend(
		m.ChannelID,
		fmt.Sprintf("definition(s) of %s:\n%s", term, strings.ReplaceAll(bodString, "\n\n", "\n")),
	)
	return
}
