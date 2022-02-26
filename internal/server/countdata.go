package server

import (
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
)

func (srv *Server) buildCountMessages(messages []discord.Message) {
	if srv.CountData == nil {
		srv.CountData = make(map[discord.UserID]map[string]int)
	}

	for _, msg := range messages {
		words := strings.Split(msg.Content, " ")
		for _, word := range words {
			word = strings.TrimSpace(word)
			if len(word) == 0 {
				continue
			}

			srv.CountMutex.Lock()
			srv.putWord(word, msg.Author.ID)
			srv.putWord(word, 0)
			srv.CountMutex.Unlock()
		}
	}
}

func (srv *Server) putWord(word string, setID discord.UserID) {
	userData, exists := srv.CountData[setID]
	if !exists {
		userData = make(map[string]int)
	}
	if _, hasWord := userData[word]; hasWord {
		userData[word]++
	} else {
		userData[word] = 1
	}

	srv.CountData[setID] = userData
}
