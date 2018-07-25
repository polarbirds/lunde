package meme

import "github.com/bwmarrin/discordgo"

type Post struct {
	Message string
	Title string
	Embed discordgo.MessageEmbed
}