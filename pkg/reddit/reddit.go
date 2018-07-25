package reddit

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/google/go-querystring/query"
	"github.com/jzelinskie/geddit"
	"github.com/polarbirds/lunde/internal/meme"
	"net/http"
	"strings"
)

var (
	imgSuffixes = []string{
		"jpg",
		"png",
	}
)

func isEmbeddable(url string) bool {
	for _, suffix := range imgSuffixes {
		if strings.HasSuffix(url, "."+suffix) {
			return true
		}
	}
	return false
}

func GetMeme(scheme string, argument string) (meme.Post, error) {
	msg := meme.Post{}
	var resp *geddit.Submission
	var err error

	resp, err = GetPost(scheme, argument)

	if err != nil {
		return msg, err
	}

	msg.Title = resp.Title
	if resp.Selftext != "" {
		msg.Message = resp.Selftext
		if len(msg.Message) > 1999 {
			msg.Message = msg.Message[:1999]
		}
	} else {
		msg.Message = resp.URL
	}

	if isEmbeddable(resp.URL) {
		a := discordgo.MessageEmbedImage{URL: resp.URL}
		msg.Embed = discordgo.MessageEmbed{
			Title: resp.Title,
			Image: &a,
		}
	}
	return msg, nil
}

func GetPost(scheme string, subreddit string) (*geddit.Submission, error) {
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

// ripped and modified from github.com/jzelinskie/geddit to support sorting random
func subredditSubmissions(subreddit string, sort string, params geddit.ListingOptions) ([]*geddit.Submission, error) {
	v, err := query.Values(params)
	if err != nil {
		return nil, err
	}

	baseUrl := "https://www.reddit.com"

	// If subbreddit given, add to URL
	if subreddit != "" {
		baseUrl += "/r/" + subreddit
	}

	redditURL := fmt.Sprintf(baseUrl+"/%s.json?%s", sort, v.Encode())

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
