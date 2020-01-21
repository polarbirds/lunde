package reddit

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/go-querystring/query"
	"github.com/jzelinskie/geddit"
	"github.com/polarbirds/lunde/internal/meme"
)

var (
	imgSuffixes = []string{
		"jpg",
		"png",
		"gif",
	}
)

func utcToTimeStamp(utc int64) string {
	tm := time.Unix(utc, 0)
	return tm.Format("02.01.06")
}

func isEmbeddable(url string) bool {
	for _, suffix := range imgSuffixes {
		if strings.HasSuffix(strings.ToLower(url), "."+suffix) {
			return true
		}
	}
	return false
}

func embedMessage(resp *geddit.Submission) discordgo.MessageEmbed {
	embed := discordgo.MessageEmbed{
		Title:       resp.Title,
		Description: resp.Selftext,
		Author: &discordgo.MessageEmbedAuthor{
			Name: resp.Author,
		},
		Timestamp: time.Unix(int64(resp.DateCreated), 0).Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("⬆%v / ⬇%v", resp.Ups, resp.Downs),
		},
	}

	if len(embed.Description) > 1999 {
		embed.Description = embed.Description[:1999]
	}

	if !strings.HasSuffix(resp.URL, resp.Permalink) {
		embed.Image = &discordgo.MessageEmbedImage{URL: resp.URL}
	}
	return embed
}

// GetMeme fetches a post from reddit from the given parameters
func GetMeme(scheme string, argument string) (msg meme.Post, err error) {
	var resp *geddit.Submission
	var subreddit string
	nrPost := 1
	splits := strings.Split(argument, " ")
	subreddit = splits[0]
	if len(splits) > 1 {
		nrPost, err = strconv.Atoi(splits[1])
		if err != nil {
			return msg, err
		}
	}
	// make 0-indexed
	nrPost--

	// fy tr0n
	if nrPost < 0 {
		nrPost = 0
	}

	resp, err = getPost(scheme, subreddit, nrPost)

	if err != nil {
		return msg, err
	}

	if strings.HasSuffix(resp.URL, resp.Permalink) || isEmbeddable(resp.URL) {
		msg.Embed = embedMessage(resp)
	} else {
		msg.Title = fmt.Sprintf("*%s on %s*:\n%s", resp.Author, utcToTimeStamp(int64(resp.DateCreated)), resp.Title)
		var messageBody string
		if resp.Selftext != "" {
			messageBody = fmt.Sprintf("%s\n%s", resp.Selftext, resp.URL)
		} else {
			messageBody = resp.URL
		}
		msg.Message = fmt.Sprintf("%s\n⬆%v / ⬇%v", messageBody, resp.Ups, resp.Downs)
	}

	return msg, nil
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
		submissions, err = subredditSubmissions("", nrPost, scheme, subOpts)
	} else {
		submissions, err = subredditSubmissions(subreddit, nrPost, scheme, subOpts)
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
	subreddit string, nrPost int, sort string, params geddit.ListingOptions,
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
