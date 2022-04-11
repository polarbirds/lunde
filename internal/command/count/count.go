package count

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/polarbirds/lunde/internal/server"
	"github.com/sirupsen/logrus"
)

type countHandler struct {
	srv *server.Server
}

type wordCount struct {
	userID discord.UserID
	word   string
	count  int
}

// CreateCommand creates a lunde command to slap people
func CreateCommand(srv *server.Server) (cmd command.LundeCommand, err error) {
	ch := countHandler{srv}

	cmd = command.LundeCommand{
		HandleInteraction: ch.handleInteraction,
		CommandData: api.CreateCommandData{
			Name:        "count",
			Description: "show most used words for users",
			Options: []discord.CommandOption{
				&discord.StringOption{
					OptionName:  "word",
					Description: "what word to show count(s) for",
					Required:    false,
				},
				&discord.UserOption{
					OptionName:  "target",
					Description: "who to show counts of words for",
					Required:    false,
				},
			},
		},
	}

	return
}

func (ch *countHandler) handleInteraction(
	_ *gateway.InteractionCreateEvent, options map[string]discord.CommandInteractionOption,
) (
	response *api.InteractionResponseData, err error,
) {
	if !ch.srv.BuildingDataDone {
		err = errors.New("building data not done, try again later")
		return
	}

	word := options["word"].String()

	target, err := options["target"].SnowflakeValue()
	if err != nil {
		err = fmt.Errorf("parsing target snowflake: %w", err)
		return
	}

	userID := discord.UserID(target)

	var title string
	var msg string
	if word != "" && target != 0 {
		// word and user defined
		title, msg, err = ch.wordCountForUser(word, userID)
	} else if word != "" && target == 0 {
		// word defined but not user
		title, msg, err = ch.topUsersForWord(word)
	} else if word == "" && target != 0 {
		// user defined but no word
		title, msg, err = ch.topWordsForUser(userID)
	} else {
		err = errors.New("invalid combination of arguments")
		return
	}

	if err != nil {
		err = fmt.Errorf("building message: %w", err)
		return
	}

	embed := discord.Embed{}
	if title != "" {
		embed.Title = title
	}
	if msg != "" {
		embed.Description = msg
	}
	embeds := []discord.Embed{embed}
	response = &api.InteractionResponseData{
		Embeds: &embeds,
	}

	return
}

func (ch *countHandler) wordCountForUser(word string, userID discord.UserID) (
	_ string, msg string, err error,
) {
	dataset, exists := ch.srv.CountData[userID]
	if !exists {
		err = fmt.Errorf("found no dataset for userID %d", userID)
		return
	}

	count, hasSaidWord := dataset[word]
	if !hasSaidWord {
		msg = fmt.Sprintf("the word `%s` has never been said by %s", word, userID.Mention())
		return
	}
	msg = fmt.Sprintf("the word `%s` has been said by %s a total of %d times",
		word, userID.Mention(), count)
	return
}

func (ch *countHandler) topWordsForUser(userID discord.UserID) (
	title string, msg string, err error,
) {
	dataset, exists := ch.srv.CountData[userID]
	if !exists {
		err = fmt.Errorf("found no dataset for userID %d", userID)
		return
	}

	title = "Top 10 words for user"

	msg = fmt.Sprintf("Top 10 words for %s:\n```", userID.Mention())
	counts := sortSetAsPairs(dataset, false)
	if len(counts) > 10 {
		counts = counts[:10]
	}

	for i, p := range counts {
		msg += fmt.Sprintf("\n%d. %s: %d", i+1, p.word, p.count)
	}

	msg += "```"
	return
}

func (ch *countHandler) topUsersForWord(word string) (
	title string, msg string, err error,
) {
	started := time.Now()
	logrus.Infof("started: %s", started.Format(time.RFC3339))

	wordCounts := []wordCount{}
	for userID, userData := range ch.srv.CountData {
		if userID == 0 {
			continue
		}
		count, hasSaidWord := userData[word]
		if !hasSaidWord {
			continue
		}
		wordCounts = append(wordCounts, wordCount{
			userID: userID,
			word:   word,
			count:  count,
		})
	}

	if len(wordCounts) == 0 {
		title = fmt.Sprintf("No one has said the word `%s` before", word)
		return
	}

	wordCountsCount := 10
	if len(wordCounts) < 10 {
		wordCountsCount = len(wordCounts)
	}

	title = fmt.Sprintf("Top %d users who have said `%s`", wordCountsCount, word)

	msg = ""
	counts := sortPairs(wordCounts, false)
	if len(counts) > wordCountsCount {
		counts = counts[:wordCountsCount]
	}

	lines := make([]string, len(counts))

	for i, p := range counts {
		lines = append(lines, fmt.Sprintf("%d. %s: %d", i+1, p.userID.Mention(), p.count))
	}

	msg = strings.Join(lines, "\n")
	return
}

func sortSetAsPairs(set map[string]int, reverse bool) []wordCount {
	var sortedSet []wordCount
	for k, v := range set {
		sortedSet = append(sortedSet, wordCount{
			word:  k,
			count: v,
		})
	}

	return sortPairs(sortedSet, reverse)
}

func sortPairs(pairs []wordCount, reverse bool) []wordCount {
	sort.Slice(pairs, func(i, j int) bool {
		if reverse {
			return pairs[i].count < pairs[j].count
		}
		return pairs[i].count > pairs[j].count
	})
	return pairs
}
