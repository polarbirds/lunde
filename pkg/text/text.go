package text

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"math/rand"
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
) (*discordgo.Message, error) {
	if !strings.HasPrefix(scheme, "-") {
		scheme = "-" + scheme
	}
	types, content := findTypes(strings.Join([]string{scheme, argument}, " "))
	log.Info(types, ": ", content)

	for _, v := range types {
		content = convert(content, v)
	}

	return s.ChannelMessageSend(m.ChannelID, content)
}

func findTypes(msg string) ([]string, string) {
	remainingMessage := msg
	types := []string{}
	words := strings.Split(msg, " ")
	for i, v := range words {
		if strings.HasPrefix(v, "-") {
			types = append(types, v)
		} else {
			remainingMessage = strings.Join(words[i:], " ")
			break
		}
	}
	return types, remainingMessage
}

func convert(content string, algo string) string {
	if strings.HasPrefix(algo, "-") {
		algo = algo[1:]
	}

	setRandSeed(content)
	switch algo {
	case "zalgo":
		return zalgoPlz(content)
	case "spunge":
		return spungePlz(content)
	default:
		return content
	}
}

func setRandSeed(content string) {
	h := fnv.New64a()
	h.Write([]byte(content))
	rand.Seed(int64(h.Sum64()))
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
	for i, c := range content {
		if rand.Intn(2) == 0 {
			content = content[:i] + strings.ToUpper(string(c)) + content[i+1:]
		}
	}
	return content
}
