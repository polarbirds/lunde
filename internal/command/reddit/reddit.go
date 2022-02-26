package reddit

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/session"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/google/go-querystring/query"
	"github.com/jzelinskie/geddit"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/polarbirds/lunde/internal/server"
)

type redditHandler struct {
	session *session.Session
}

// CreateCommand creates a LundeCommand which handles /reddit
func CreateCommand(srv *server.Server) (cmd command.LundeCommand, err error) {
	rh := redditHandler{srv.Session}
	cmd = command.LundeCommand{
		HandleInteraction: rh.handleInteraction,
		CommandData: api.CreateCommandData{
			Name: "reddit",
			Description: "fetches reddit posts the given subreddit sorted by the given " +
				"parameters",
			Options: []discord.CommandOption{
				&discord.StringOption{
					OptionName:  "sort",
					Description: "what algorithm to sort posts by",
					Required:    true,
					Choices: []discord.StringChoice{
						{Name: "top", Value: "top"},
						{Name: "hot", Value: "hot"},
						{Name: "controversial", Value: "controversial"},
						{Name: "random", Value: "random"},
					},
				},
				&discord.StringOption{
					OptionName:  "sub",
					Description: "what subreddit to fetch from",
					Required:    true,
				},
			},
		},
	}

	return
}

func utcToTimeStamp(utc int64) string {
	tm := time.Unix(utc, 0)
	return tm.Format("02.01.06")
}

func isEmbeddable(url string) bool {
	urlDotSplits := strings.Split(strings.ToLower(url), ".")
	switch urlDotSplits[len(urlDotSplits)-1] {
	case "jpg", "png", "gif":
		return true
	}

	return false
}

func embedMessage(resp *geddit.Submission) discord.Embed {
	embed := discord.Embed{
		Title:       resp.Title,
		Description: resp.Selftext,
		Author: &discord.EmbedAuthor{
			Name: resp.Author,
		},
		URL:       fmt.Sprintf("https://reddit.com%s", resp.Permalink),
		Timestamp: discord.NewTimestamp(time.Unix(int64(resp.DateCreated), 0)),
		Footer: &discord.EmbedFooter{
			Text: fmt.Sprintf("⬆%v", resp.Ups),
			// revive:disable-next-line:line-length-limit
			Icon: "https://b.thumbs.redditmedia.com/S6FTc5IJqEbgR3rTXD5boslU49bEYpLWOlh8-CMyjTY.png",
		},
	}

	if len(embed.Description) > 1999 {
		embed.Description = embed.Description[:1999]
	}

	if !strings.HasSuffix(resp.URL, resp.Permalink) {
		embed.Image = &discord.EmbedImage{URL: resp.URL}
	}
	return embed
}

func (rh *redditHandler) handleInteraction(
	event *gateway.InteractionCreateEvent,
	options map[string]string,
) (
	response *api.InteractionResponseData, err error,
) {
	var recChan *discord.Channel
	recChan, err = rh.session.Channel(event.ChannelID)
	if err != nil {
		err = fmt.Errorf("get channel when handling /reddit: %v", err)
		return
	}

	var resp *geddit.Submission
	var subreddit string
	nrPost := 1
	splits := strings.Split(options["sub"], " ")
	subreddit = splits[0]
	if len(splits) > 1 {
		nrPost, err = strconv.Atoi(splits[1])
		if err != nil {
			return
		}
	}
	// make 0-indexed
	nrPost--

	// fy tr0n
	if nrPost < 0 {
		nrPost = 0
	}

	resp, err = getPost(options["sort"], subreddit, nrPost)
	if err != nil {
		return
	}

	if resp.IsNSFW && !recChan.NSFW {
		nick := event.Member.Nick
		if nick == "" {
			nick = event.Member.User.Username
		}
		response = &api.InteractionResponseData{
			Content: option.NewNullableString(
				fmt.Sprintf("this is a christian channel, %s", nick)),
		}
		return
	}

	response = &api.InteractionResponseData{}
	if strings.HasSuffix(resp.URL, resp.Permalink) || isEmbeddable(resp.URL) {
		response.Embeds = &[]discord.Embed{embedMessage(resp)}
	} else {
		title := fmt.Sprintf("*%s on %s*: <https://reddit.com%s>\n%s",
			resp.Author, utcToTimeStamp(int64(resp.DateCreated)), resp.Permalink, resp.Title)
		var messageBody string
		if resp.Selftext != "" {
			messageBody = fmt.Sprintf("%s\n%s", resp.Selftext, resp.URL)
		} else {
			messageBody = resp.URL
		}
		response.Content = option.NewNullableString(
			fmt.Sprintf("%s\n%s\n⬆%v", title, messageBody, resp.Ups))
	}

	return
}

func getPost(scheme string, subreddit string, nrPost int) (*geddit.Submission, error) {
	// Set listing options
	subOpts := geddit.ListingOptions{
		Limit: nrPost + 1,
		Count: nrPost + 1,
	}

	var submissions []*geddit.Submission
	var err error

	if subreddit == "" {
		submissions, err = subredditSubmissions("", scheme, subOpts)
	} else {
		submissions, err = subredditSubmissions(subreddit, scheme, subOpts)
	}

	if err != nil {
		return nil, err
	}

	if len(submissions) < 1 {
		return nil, fmt.Errorf("reddit returned no posts for subreddit %q", subreddit)
	}

	if len(submissions)-1 < nrPost {
		return nil, fmt.Errorf("reddit did not return enough posts. "+
			"Reddit returned %d post(s) for subreddit %q, user requested post #%d",
			len(submissions), subreddit, nrPost)
	}

	return submissions[nrPost], nil
}

// ripped from github.com/jzelinskie/geddit, but now supports all of reddit's sorting-algorithms
func subredditSubmissions(
	subreddit string, sort string, params geddit.ListingOptions,
) ([]*geddit.Submission, error) {
	v, err := query.Values(params)
	if err != nil {
		return nil, err
	}

	baseURL := "https://www.reddit.com"

	// If subbreddit given, add to URL
	if subreddit != "" {
		baseURL += "/r/" + subreddit
	}

	redditURL := fmt.Sprintf(baseURL+"/%s.json?%s", sort, v.Encode())

	client := &http.Client{}
	req, err := http.NewRequest("GET", redditURL, nil)
	req.Header.Set("User-Agent", "lunde")
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	type Response struct {
		Data struct {
			Children []struct {
				Data *geddit.Submission
			}
		}
	}

	var r Response

	if sort == "random" {
		var postsRes []Response
		err = json.NewDecoder(resp.Body).Decode(&postsRes)
		if err != nil {
			return nil, err
		}

		if len(postsRes) < 1 {
			return nil, errors.New("invalid subreddit")
		}
		r = postsRes[0]
	} else {
		var postsRes Response
		err = json.NewDecoder(resp.Body).Decode(&postsRes)
		if err != nil {
			return nil, err
		}

		r = postsRes
	}

	submissions := make([]*geddit.Submission, len(r.Data.Children))
	for i, child := range r.Data.Children {
		submissions[i] = child.Data
	}

	return submissions, nil
}
