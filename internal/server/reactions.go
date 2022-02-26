package server

import (
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/sirupsen/logrus"
)

// HandleReactionAddInteraction handles when reactions are added to messages
func (srv *Server) HandleReactionAddInteraction(ev *gateway.MessageReactionAddEvent) {
	logrus.Infof("%s %+v", ev.Member.Nick, ev)
}
