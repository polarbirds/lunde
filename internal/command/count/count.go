package count

import (
	"errors"
	"fmt"
	"sort"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/polarbirds/lunde/internal/command"
	"github.com/polarbirds/lunde/internal/server"
)

type countHandler struct {
	srv *server.Server
}

type pair struct {
	name  string
	count int
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
					Description: "what word to check for",
					Required:    false,
				},
				&discord.UserOption{
					OptionName:  "target",
					Description: "who to check words for",
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

	var user *discord.User
	target, err := options["target"].SnowflakeValue()
	if err != nil {
		err = fmt.Errorf("parsing target ID: %w", err)
		return
	}
	user, err = ch.srv.Session.User(discord.UserID(target))
	if err != nil {
		err = fmt.Errorf("get user: %w", err)
		return
	}

	var msg string
	if word != "" && !target.IsNull() {
		msg, err = ch.wordCountForUser(word, user)
	} else if word != "" && target.IsNull() {
		msg, err = ch.topUsersForWord(word)
	} else if word == "" && !target.IsNull() {
		msg, err = ch.topWordsForUser(user)
	}

	if err != nil {
		err = fmt.Errorf("building message: %w", err)
		return
	}

	response = &api.InteractionResponseData{
		Content: option.NewNullableString(msg),
	}

	return
}

func (ch *countHandler) wordCountForUser(word string, user *discord.User) (
	msg string, err error,
) {
	dataset, exists := ch.srv.CountData[user.ID]
	if !exists {
		err = fmt.Errorf("found no dataset for userID %d", user.ID)
		return
	}

	count, hasSaidWord := dataset[word]
	if !hasSaidWord {
		msg = fmt.Sprintf("the word %s has never been said by %s", word, user.Mention())
		return
	}
	msg = fmt.Sprintf("the word %s has been said by %s a total of %d times",
		word, user.Mention(), count)
	return
}

func (ch *countHandler) topWordsForUser(user *discord.User) (
	msg string, err error,
) {
	dataset, exists := ch.srv.CountData[user.ID]
	if !exists {
		err = fmt.Errorf("found no dataset for userID %d", user.ID)
		return
	}

	msg = fmt.Sprintf("Top 10 words for %s:\n```", user.Username)
	counts := sortSetAsPairs(dataset, false)
	if len(counts) > 10 {
		counts = counts[:10]
	}

	for i, p := range counts {
		msg += fmt.Sprintf("\n%d. %s: %d", i+1, p.name, p.count)
	}

	msg += "```"
	return
}

func (ch *countHandler) topUsersForWord(word string) (
	msg string, err error,
) {
	usersCounts := []pair{}
	for userID, userData := range ch.srv.CountData {
		var user *discord.User
		user, err = ch.srv.Session.User(userID)
		if err != nil {
			err = fmt.Errorf("get user for userID %q: %w", userID, err)
			return
		}
		count, hasSaidWord := userData[word]
		if !hasSaidWord {
			continue
		}
		usersCounts = append(usersCounts, pair{
			name:  user.Username,
			count: count,
		})
	}

	if len(usersCounts) == 0 {
		msg = fmt.Sprintf("No one has said the word %q before", word)
		return
	}

	msg = fmt.Sprintf("Top 10 users who have said %q:\n```", word)
	counts := sortPairs(usersCounts, false)
	if len(counts) > 10 {
		counts = counts[:10]
	}

	for i, p := range counts {
		msg += fmt.Sprintf("\n%d. %s: %d", i+1, p.name, p.count)
	}

	msg += "```"
	return
}

func sortSetAsPairs(set map[string]int, reverse bool) []pair {
	var sortedSet []pair
	for k, v := range set {
		sortedSet = append(sortedSet, pair{k, v})
	}

	return sortPairs(sortedSet, reverse)
}

func sortPairs(pairs []pair, reverse bool) []pair {
	sort.Slice(pairs, func(i, j int) bool {
		if reverse {
			return pairs[i].count < pairs[j].count
		}
		return pairs[i].count > pairs[j].count
	})
	return pairs
}
