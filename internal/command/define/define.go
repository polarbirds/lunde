package define

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
)

type ubResponse struct {
	List []struct {
		Word       string `json:"word"`
		Definition string `json:"definition"`
		Example    string `json:"example"`
		ThumbsUp   int    `json:"thumbs_up"`
		ThumbsDown int    `json:"thumbs_down"`
	} `json:"list"`
}

// CommandData returns request
func CommandData() api.CreateCommandData {
	return api.CreateCommandData{
		Name:        "define",
		Description: "fetch a totally legit definition from a very reputable and renowned source",
		Options: []discord.CommandOption{
			{
				Name:        "term",
				Type:        discord.StringOption,
				Description: "term to fetch definition for",
				Required:    true,
			},
		},
	}
}

// HandleDefine fetches and sends a definition for the given term
func HandleDefine(term string) (reply *api.InteractionResponseData, err error) {
	var res *http.Response
	res, err = http.Get("http://api.urbandictionary.com/v0/define?term=" + url.QueryEscape(term))
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

	var ubRes ubResponse
	err = json.Unmarshal(bodBytes, &ubRes)
	if err != nil {
		err = fmt.Errorf("no definitions found")
		return
	}

	if len(ubRes.List) == 0 {
		err = fmt.Errorf("no definition returned for the term: %s", term)
		return
	}

	replyContent := fmt.Sprintf("Definition(s) for **%s**:", term)
	for i := 0; i < 3 && i < len(ubRes.List); i++ {
		def := ubRes.List[i]
		replyContent += fmt.Sprintf("\n%d) **%s**: %s ⬆%v / ⬇%v\n%s\n",
			i+1, def.Word, sanitizeUrbanDictionaryText(def.Definition),
			def.ThumbsUp, def.ThumbsDown,
			sanitizeUrbanDictionaryText(def.Example))
	}

	reply = &api.InteractionResponseData{
		Content: option.NewNullableString(replyContent),
	}

	return
}

func sanitizeUrbanDictionaryText(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "]", ""), "[", "")
}
