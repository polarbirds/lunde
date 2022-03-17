package channelnames

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/polarbirds/lunde/internal/server"
	"github.com/sirupsen/logrus"
	"gopkg.in/robfig/cron.v2"
)

const fridayAt16CronPattern = "0 16 * * 5"
const mondayAt01CronPattern = "0 1 * * 1"

type channelNamesRunner struct {
	srv *server.Server
}

// StartScheduler starts the monday/friday string replacer
func StartScheduler(srv *server.Server) (err error) {
	wnr := channelNamesRunner{srv}

	cron := cron.New()
	_, err = cron.AddFunc(fridayAt16CronPattern, wnr.createChannelNamesReplacer("☕", "🍻"))
	if err != nil {
		err = fmt.Errorf("create replacer for fridays: %v", err)
		return
	}

	_, err = cron.AddFunc(mondayAt01CronPattern, wnr.createChannelNamesReplacer("🍻", "☕"))
	if err != nil {
		err = fmt.Errorf("create replacer for mondays: %v", err)
		return
	}

	_, err = cron.AddFunc(mondayAt01CronPattern, wnr.channelnameReplaceNM)
	if err != nil {
		err = fmt.Errorf("create replacer for mondays: %v", err)
		return
	}

	cron.Start()

	return
}

func (wnr *channelNamesRunner) createChannelNamesReplacer(from string, to string) func() {
	return func() {
		err := wnr.replaceChannelNames(from, to)
		if err != nil {
			logrus.Errorf("error occurred replacing %q with %q in channel names: %v", from, to, err)
			return
		}
		logrus.Infof("replaced character %q with %q in channel names", from, to)
	}
}

func (wnr *channelNamesRunner) channelnameReplaceNM() {
	if rand.Float32() > 0.1 {
		return
	}
	err := wnr.replaceChannelNames("n", "m")
	if err != nil {
		logrus.Errorf("error occurred replacing n with m in channel names: %v", err)
		return
	}
	err = wnr.replaceChannelNames("N", "M")
	if err != nil {
		logrus.Errorf("error occurred replacing N with M in channel names: %v", err)
		return
	}
	logrus.Infof("replaced character n/N with m/M in channel names")
}

func (wnr *channelNamesRunner) replaceChannelNames(from string, to string) error {
	chans, err := wnr.srv.Session.Channels(wnr.srv.GuildID)
	if err != nil {
		return fmt.Errorf("getting channel names; %v", err)
	}

	errs := []error{}
	for _, ch := range chans {
		err = wnr.replaceChannelName(ch, from, to)
		if err != nil {
			logrus.Errorf("error occurred replacing %q with %q in channel %q (ID is %d): %v",
				from, to, ch.Name, ch.ID, err)
			errs = append(errs, err)
			continue
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors occurred updating channels: %v", errs)
	}
	return nil
}

func (wnr *channelNamesRunner) replaceChannelName(
	ch discord.Channel,
	from string,
	to string,
) error {
	if !strings.Contains(ch.Name, from) {
		return nil
	}

	err := wnr.srv.Session.ModifyChannel(ch.ID, api.ModifyChannelData{
		Name: strings.ReplaceAll(ch.Name, from, to),
	})
	if err != nil {
		return fmt.Errorf("error occurred updating channel by name %q and ID %s: %v",
			ch.Name, ch.ID, err)
	}

	return nil
}
