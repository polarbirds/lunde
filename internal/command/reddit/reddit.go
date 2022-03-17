package reddit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/session"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/jzelinskie/geddit"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/polarbirds/lunde/internal/server"
)

type redditHandler struct {
	session *session.Session
}

func intToPtr(i int) *int {
	return &i
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
				&discord.IntegerOption{
					OptionName:  "offset",
					Description: "how many posts to skip",
					Required:    false,
					Min:         option.Int(intToPtr(0)),
					Max:         option.Int(intToPtr(200)),
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
	options map[string]discord.CommandInteractionOption,
) (
	response *api.InteractionResponseData, err error,
) {
	var recChan *discord.Channel
	recChan, err = rh.session.Channel(event.ChannelID)
	if err != nil {
		err = fmt.Errorf("get channel when handling /reddit: %v", err)
		return
	}

	subreddit := options["sub"].String()

	offset, err := options["offset"].IntValue()
	if err != nil {
		err = fmt.Errorf("parsing int option: %v", err)
		return
	}

	// fy tr0n
	if offset < 0 {
		offset = 0
	}

	resp, err := getPost(options["sort"].String(), subreddit, offset)
	if err != nil {
		err = fmt.Errorf("getting post: %v", err)
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

func getPost(scheme string, subreddit string, offset int64) (*geddit.Submission, error) {
	submissions, err := getSubredditSubmissions(subreddit, scheme, offset+1, offset+1)
	if err != nil {
		return nil, fmt.Errorf("getting submissions: %v", err)
	}

	if len(submissions) < 1 {
		return nil, fmt.Errorf("reddit returned no posts for subreddit %q", subreddit)
	}

	if len(submissions)-1 < int(offset) {
		return nil, fmt.Errorf("reddit did not return enough posts. "+
			"Reddit returned %d post(s) for subreddit %q, user requested post #%d",
			len(submissions), subreddit, offset)
	}

	return submissions[offset], nil
}

func getSubredditSubmissions(
	subreddit string, sort string, count int64, limit int64,
) ([]*geddit.Submission, error) {
	redditURL := fmt.Sprintf(
		"https://www.reddit.com/r/%s/%s.json?count=%d&limit=%d",
		subreddit, sort, count, limit)

	req, err := http.NewRequest("GET", redditURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %v", err)
	}

	req.Header.Set("User-Agent", "linux:lunde:1.0.0 (by /u/haraldfw) ")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doing request: %v", err)
	}

	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("reddit returned unexpected non-200 response code %q", resp.Status)
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
		err = json.Unmarshal(bodyBytes, &postsRes)
		if err != nil {
			return nil, fmt.Errorf("decoding response %q: %v", string(bodyBytes), err)
		}

		if len(postsRes) < 1 {
			return nil, errors.New("no posts returned for subreddit")
		}
		r = postsRes[0]
	} else {
		var postRes Response
		err = json.Unmarshal(bodyBytes, &postRes)
		if err != nil {
			return nil, fmt.Errorf("decoding response body %q: %v", string(bodyBytes), err)
		}

		r = postRes
	}

	submissions := make([]*geddit.Submission, len(r.Data.Children))
	for i, child := range r.Data.Children {
		submissions[i] = child.Data
	}

	return submissions, nil
}
