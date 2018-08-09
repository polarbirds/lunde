package xkcd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/polarbirds/lunde/internal/meme"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
)

type XKCD struct {
	Img   string
	Title string
	Alt   string
	Num   int
}

type SXImg struct {
	Number    int
	Title     string
	Titletext string
	Image     string
}

type SearchXKCD struct {
	Success bool
	Results []SXImg
}

func GetMeme(scheme string, argument string) (meme.Post, error) {
	switch scheme {
	case "random":
		return getRandom()
	case "search":
		return getSearch(argument)
	default:
		return getNewestXKCD()
	}
}

func getSearch(searchString string) (meme.Post, error) {
	m := meme.Post{}
	r, err := http.Post(
		"https://relevant-xkcd-backend.herokuapp.com/search",
		"application/x-www-form-urlencoded; charset=UTF-8",
		bytes.NewReader([]byte("search="+strings.Replace(searchString, " ", "+", -1))),
	)
	if err != nil {
		return m, err
	}
	bod, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return m, err
	}
	n := SearchXKCD{}
	err = json.Unmarshal(bod, &n)
	if err != nil {
		return m, err
	}

	if len(n.Results) == 0 {
		return m, errors.New("no results")
	}

	res := n.Results[0]

	img := discordgo.MessageEmbedImage{
		URL: res.Image,
	}
	m.Embed = discordgo.MessageEmbed{
		Title:       res.Title,
		Image:       &img,
		Description: res.Titletext,
	}
	return m, nil
}

func getNewestXKCD() (meme.Post, error) {
	m := meme.Post{}
	x, err := queryXKCDAPI(0)
	if err != nil {
		return m, err
	}
	xkcdToPost(x, &m)
	return m, nil
}

func queryXKCDAPI(num int) (XKCD, error) {
	x := XKCD{}
	var r *http.Response
	var err error
	if num > 0 {
		r, err = http.Get(fmt.Sprintf("http://xkcd.com/%d/info.0.json", num))
	} else {
		r, err = http.Get("http://xkcd.com/info.0.json")
	}
	if err != nil {
		return x, err
	}
	bod, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return x, err
	}
	err = json.Unmarshal(bod, &x)
	if err != nil {
		return x, err
	}
	return x, nil
}

func getRandom() (meme.Post, error) {
	m := meme.Post{}

	//
	//r, err := http.Get("https://c.xkcd.com/random/comic/")
	//if err != nil {
	//	return m, err
	//}

	x, err := queryXKCDAPI(0)
	if err != nil {
		return m, err
	}

	x, err = queryXKCDAPI(rand.Intn(x.Num) + 1)
	if err != nil {
		return m, err
	}
	xkcdToPost(x, &m)
	return m, nil
}

func xkcdToPost(x XKCD, m *meme.Post) {
	img := discordgo.MessageEmbedImage{
		URL: x.Img,
	}
	m.Embed = discordgo.MessageEmbed{
		Title:       x.Title,
		Image:       &img,
		Description: x.Alt,
	}
}
