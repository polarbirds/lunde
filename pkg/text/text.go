package text

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/kortschak/zalgo"
	log "github.com/sirupsen/logrus"
)

func Generate(
	s *discordgo.Session,
	m *discordgo.MessageCreate,
	scheme string,
	argument string,
	lastMessage string,
) (*discordgo.Message, error) {
	if !strings.HasPrefix(scheme, "-") {
		scheme = "-" + scheme
	}
	types, content := findTypes(strings.Join([]string{scheme, argument}, " "))
	if content == "" {
		content = lastMessage
	}
	log.Infof("%v: %q", types, content)
	if content == "" {
		return nil, errors.New("empty message")
	}

	for _, v := range types {
		content = convert(content, v)
	}

	return s.ChannelMessageSend(m.ChannelID, content)
}

func findTypes(msg string) ([]string, string) {
	types := []string{}
	words := strings.Split(msg, " ")
	remainingMessage := words
	for i, v := range words {
		if strings.HasPrefix(v, "-") {
			types = append(types, v)
			remainingMessage = words[i+1:]
		} else {
			remainingMessage = words[i:]
			break
		}
	}
	return types, strings.Join(remainingMessage, " ")
}

func convert(content string, algo string) string {
	if strings.HasPrefix(algo, "-") {
		algo = algo[1:]
	}

	switch algo {
	case "zalgo":
		return zalgoPlz(content)
	case "spunge":
		return spungePlz(content)
	case "chirese":
		return chiresePlz(content)
	default:
		return content
	}
}

func zalgoPlz(content string) string {
	w := bytes.NewBufferString("")
	z := zalgo.NewCorrupter(w)

	z.Zalgo = func(n int, r rune, z *zalgo.Corrupter) bool {
		z.Up += 0.1
		z.Middle += complex(0.01, 0.01)
		z.Down += complex(real(z.Down)*0.1, 0)
		return false
	}

	z.Up = complex(0, 0.2)
	z.Middle = complex(0, 0.2)
	z.Down = complex(0.001, 0.3)

	fmt.Fprintf(z, content)
	return w.String()
}

func spungePlz(content string) string {
	content = strings.ToLower(content)
	lastCharConverted := false
	for i, c := range content {
		setSize := 2
		if lastCharConverted {
			setSize = 3
		}
		if rand.Intn(setSize) == 0 {
			content = content[:i] + strings.ToUpper(string(c)) + content[i+1:]
			lastCharConverted = true
		} else {
			lastCharConverted = false
		}
	}
	return content
}

var chiresePattern = regexp.MustCompile("asd")

const tmp = "||tmp||"

func swap(src string, c1 string, c2 string) string {
	src = strings.Replace(src, c1, tmp, -1) // c1 -> tmp
	src = strings.Replace(src, c2, c1, -1)  // c2 -> c1
	src = strings.Replace(src, tmp, c2, -1) // tmp -> c2

	return src
}

func chiresePlz(content string) string {
	content = swap(content, "l", "r")
	content = swap(content, "L", "R")
	return content
}
