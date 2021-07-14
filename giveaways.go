package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func findHost(s *discordgo.Session, m *discordgo.MessageCreate) string {
	giveaway := reGiveawayMessage.FindStringSubmatch(m.Content)

	var giveawayID string
	if len(giveaway) > 1 {
		giveawayID = giveaway[3]
	} else {
		giveawayID = m.Message.ID
	}

	messages, _ := s.ChannelMessages(m.ChannelID, 100, "", "", giveawayID)
	messages2, _ := s.ChannelMessages(m.ChannelID, 100, "", "", messages[len(messages)-1].ID)
	messages3, _ := s.ChannelMessages(m.ChannelID, 100, "", "", messages2[len(messages2)-1].ID)
	messages = append(messages, messages2...)
	messages = append(messages, messages3...)

	reGiveawayHost := regexp.MustCompile("Hosted by: .*003c@([0-9]+).*003e")

	for i := len(messages) - 1; i >= 0; i-- {
		content, _ := json.Marshal(messages[i])
		if reGiveawayHost.Match(content) {
			host := reGiveawayHost.FindStringSubmatch(string(content))[1]
			return host
		}
	}
	return ""
}

func handleNewGiveaway(s *discordgo.Session, m *discordgo.MessageCreate) {
	content, _ := json.Marshal(m)
	reUsername := regexp.MustCompile(`username":"[a-zA-Z0-9]+"`)
	reBot := regexp.MustCompile(`"bot":(true|false)`)
	content = []byte(reUsername.ReplaceAllString(string(content), ""))
	content = []byte(reBot.ReplaceAllString(string(content), ""))

	if len(settings.Giveaway.BlacklistWords) > 0 {
		for _, word := range settings.Giveaway.BlacklistWords {
			if strings.Contains(strings.ToLower(string(content)), strings.ToLower(word)) {
				return
			}
		}
	}

	if len(settings.Giveaway.WhitelistWords) > 0 {
		for i, word := range settings.Giveaway.WhitelistWords {
			if strings.Contains(strings.ToLower(string(content)), strings.ToLower(word)) {
				break
			}
			if i == len(settings.Giveaway.WhitelistWords)-1 {
				return
			}
		}
	}

	time.Sleep(time.Duration(settings.Giveaway.Delay) * time.Second)
	guild, err := s.State.Guild(m.GuildID)
	if err != nil || guild == nil {
		guild, err = s.Guild(m.GuildID)
		if err != nil {
			println()
			return
		}
	}

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil || guild == nil {
		channel, err = s.Channel(m.ChannelID)
		if err != nil {
			println()
			return
		}
	}
	logWithTime("[-] " + s.State.User.Username + " entered a Giveaway</><magenta> [" + guild.Name + " > " + channel.Name + "]</>")
	_ = s.MessageReactionAdd(m.ChannelID, m.ID, "🎉")
}

func handleGiveawayWon(s *discordgo.Session, m *discordgo.MessageCreate) {
	won := reGiveaway.FindStringSubmatch(m.Content)
	giveawayID := reGiveawayMessage.FindStringSubmatch(m.Content)
	guild, err := s.State.Guild(m.GuildID)
	if err != nil || guild == nil {
		guild, err = s.Guild(m.GuildID)
		if err != nil {
			println()
			return
		}
	}

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil || guild == nil {
		channel, err = s.Channel(m.ChannelID)
		if err != nil {
			println()
			return
		}
	}

	if giveawayID == nil {
		if len(won) > 1 {
			logWithTime("<green>[+] " + s.State.User.Username + " Won Giveaway</><magenta> [" + guild.Name + " > " + channel.Name + "]</>")
			webhookGiveaway(won[1], s.State.User, guild.Name, channel.Name)
		}
		webhookGiveaway("", s.State.User, guild.Name, channel.Name)
	} else {
		if len(won) > 1 {
			webhookGiveaway(won[1], s.State.User, guild.Name, channel.Name)
			logWithTime("<green>[+] " + s.State.User.Username + " Won Giveaway: </>" + won[1] + "<magenta> [" + guild.Name + " > " + channel.Name + "]</>")
		} else {
			logWithTime("<green>[+] " + s.State.User.Username + " Won Giveaway</><magenta> [" + guild.Name + " > " + channel.Name + "]</>")
			webhookGiveaway("", s.State.User, guild.Name, channel.Name)
		}
	}

	if settings.Giveaway.DM != "" {
		var giveawayHost = findHost(s, m)
		if giveawayHost == "" {
			logWithTime("<red>[x] Couldn't determine giveaway host </><magenta> [" + guild.Name + " > " + channel.Name + "]</>")
			return
		}
		hostChannel, err := s.UserChannelCreate(giveawayHost)

		if err != nil {
			return
		}
		time.Sleep(time.Second * time.Duration(settings.Giveaway.DMDelay))

		_, err = s.ChannelMessageSend(hostChannel.ID, settings.Giveaway.DM)
		if err != nil {
			return
		}

		host, _ := s.User(giveawayHost)
		logWithTime("<green>[+] " + s.State.User.Username + " sent DM to host: </>" + host.String())
	}
}
