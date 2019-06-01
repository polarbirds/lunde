package reddit

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/google/go-querystring/query"
	"github.com/jzelinskie/geddit"
	"github.com/polarbirds/lunde/internal/meme"
)

var (
	imgSuffixes = []string{
		"jpg",
		"png",
	}
)

func isEmbeddable(url string) bool {
	for _, suffix := range imgSuffixes {
		if strings.HasSuffix(strings.ToLower(url), "."+suffix) {
			return true
		}
	}
	return false
}

// GetMeme fetches a post from reddit from the given parameters
func GetMeme(scheme string, argument string) (meme.Post, error) {
	msg := meme.Post{}
	var resp *geddit.Submission
	var err error

	resp, err = getPost(scheme, argument)

	if err != nil {
		return msg, err
	}

	msg.Embed = discordgo.MessageEmbed{
		Title:       resp.Title,
		Description: resp.Selftext,
		Author: &discordgo.MessageEmbedAuthor{
			Name: resp.Author,
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("⬆%v / ⬇%v", resp.Ups, resp.Downs),
		},
	}

	if len(msg.Embed.Description) > 1999 {
		msg.Embed.Description = msg.Embed.Description[:1999]
	}

	if isEmbeddable(resp.URL) {
		a := discordgo.MessageEmbedImage{URL: resp.URL}
		msg.Embed.Image = &a
	} else if resp.Selftext == "" {
		msg.Embed.Description = resp.URL
	}

	return msg, nil
}

func getPost(scheme string, subreddit string) (*geddit.Submission, error) {
	// Set listing options
	subOpts := geddit.ListingOptions{
		Limit: 1,
		Count: 1,
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
		return nil, errors.New("reddit returned no posts")
	}

	return submissions[0], nil
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
