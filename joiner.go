package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/valyala/fasthttp"
)

func handleInviteLink(s *discordgo.Session, m *discordgo.MessageCreate) {
	if s.Token == settings.Tokens.Main || !InviteRunning {
		return
	}
	code := reInviteLink.FindStringSubmatch(m.Content)[1]
	var f = join(code, s, m)
	n := rand.Intn(settings.Invite.Delay.Max - settings.Invite.Delay.Min)
	time.AfterFunc(time.Minute*(time.Duration(settings.Invite.Delay.Min)+time.Duration(n)), f)
}

func joinServer(code string, s *discordgo.Session, m *discordgo.MessageCreate) {
	if !InviteRunning {
		return
	}
	strRequestURI := "https://discord.com/api/v8/invites/" + code
	req := fasthttp.AcquireRequest()
	req.Header.Set("authorization", s.Token)
	req.Header.SetMethodBytes([]byte("POST"))
	req.SetRequestURIBytes([]byte(strRequestURI))
	res := fasthttp.AcquireResponse()

	if err := fasthttp.Do(req, res); err != nil {
		return
	}

	fasthttp.ReleaseRequest(req)

	body := res.Body()

	if !strings.Contains(string(body), "new_member") {
		return
	}

	if !reInviteServer.Match(body) {
		return
	}

	InviteSniped++
	var serverName = reInviteServer.FindStringSubmatch(string(body))[1]

	guild, err := s.State.Guild(m.GuildID)
	if err != nil || guild == nil {
		guild, err = s.Guild(m.GuildID)
		if err != nil {
			println()
			if InviteSniped >= settings.Invite.InviteMax {
				InviteRunning = false
				logWithTime("<yellow>[+] Stopping Invite sniping for now</>")
				time.AfterFunc(time.Hour*time.Duration(settings.Invite.Cooldown), inviteTimerEnd)
			}
			return
		}
	}

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil || guild == nil {
		channel, err = s.Channel(m.ChannelID)
		if err != nil {
			println()
			if InviteSniped >= settings.Invite.InviteMax {
				InviteRunning = false
				logWithTime("<yellow>[+] Stopping Invite sniping for now</>")
				time.AfterFunc(time.Hour*time.Duration(settings.Invite.Cooldown), inviteTimerEnd)
			}
		}
	}

	logWithTime(fmt.Sprintf("<green>%s joined the server %s from %s</><magenta> ["+guild.Name+" > "+channel.Name+"]", m.Author.Username, serverName, m.Author.String()))

	if InviteSniped >= settings.Invite.InviteMax {
		InviteRunning = false
		logWithTime("<yellow>[+] Stopping Invite sniping for now</>")
		time.AfterFunc(time.Hour*time.Duration(settings.Invite.Cooldown), inviteTimerEnd)
	}
}

func join(code string, s *discordgo.Session, m *discordgo.MessageCreate) func() {
	return func() {
		joinServer(code, s, m)
	}
}
