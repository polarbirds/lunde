package server

import (
	"regexp"
	"sync"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/sirupsen/logrus"
)

var sanitizer = regexp.MustCompile("[`\\[\\]\\{\\}\\(\\)\\?'\",\\.&]")

func (srv *Server) buildData() {
	startTime := time.Now()

	chans, err := srv.Session.Channels(srv.GuildID)
	if err != nil {
		logrus.Errorf("build data: failed getting channels: %v", err)
		return
	}

	logrus.Infof("fetching messages for %d channels", len(chans))

	wg := sync.WaitGroup{}
	for ich := range chans {
		wg.Add(1)
		go func(ch discord.Channel) {
			srv.buildDataForChannel(ch)
			wg.Done()
		}(chans[ich])
	}

	wg.Wait()
	srv.BuildingDataDone = true
	logrus.Infof("done building data, took %s", time.Since(startTime))
}

func (srv *Server) buildDataForChannel(ch discord.Channel) {
	switch ch.Type {
	case discord.GuildText, discord.GroupDM, discord.DirectMessage:
	default:
		return
	}
	logrus.Infof("fetching messages for channel %s", ch.Name)
	messages, err := srv.Session.Messages(ch.ID, srv.MessagesToGetForDataBuild)
	if err != nil {
		logrus.Errorf("error occurred getting messages for channel %s: %v", ch.Name, err)
		return
	}

	logrus.Infof("found %d messages for channel %s", len(messages), ch.Name)

	srv.buildDataFromMessages(messages)
}

func (srv *Server) buildDataFromMessages(messages []discord.Message) {
	srv.buildCountMessages(messages)
}
