package main

import (
	"log"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/bwmarrin/discordgo"
	"github.com/valyala/fasthttp"
)

func checkPrivnote(s *discordgo.Session, m *discordgo.MessageCreate) {
	var link = rePrivnote.FindStringSubmatch(m.Content)
	var strRequestURI = link[1]
	var password = link[2]

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
	logWithTime("<green>[-] " + s.State.User.Username + " sniped Privnote: " + rePrivnote.FindStringSubmatch(m.Content)[0] + "</> from " + m.Author.String() + " [" + guild.Name + " > " + channel.Name + "]")

	req := fasthttp.AcquireRequest()
	req.Header.SetMethodBytes([]byte("DELETE"))
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.SetRequestURIBytes([]byte(strRequestURI))
	res := fasthttp.AcquireResponse()

	if err := fasthttp.Do(req, res); err != nil {
		return
	}

	fasthttp.ReleaseRequest(req)

	body := res.Body()

	if !rePrivnoteData.Match(body) {
		logWithTime("<red>[x] Privnote already destroyed</>")
		return
	}
	var cryptData = rePrivnoteData.FindStringSubmatch(string(body))[1]

	var cryptBytes, _ = Base64Decode([]byte(cryptData))

	var salt = cryptBytes[8:16]
	cryptBytes = cryptBytes[16:]

	key, iv := openSSLKey([]byte(password), salt)
	data := Ase256(cryptBytes, key, iv)
	if reGiftLink.Match([]byte(data)) && SniperRunning {
		code := reGiftLink.FindStringSubmatch(data)
		logWithTime("<green>[+] Found a gift link in it: </><red>" + code[2] + "</>")
		checkGiftLink(s, m, data, time.Now())
	} else {
		f, err := os.OpenFile("privnotes.txt",
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

		if err != nil {
			log.Fatal(err)
		}

		defer f.Close()

		clean := strings.Map(func(r rune) rune {
			if unicode.IsGraphic(r) {
				return r
			}
			return -1
		}, data)

		clean = strings.Map(func(r rune) rune {
			if unicode.IsPrint(r) {
				return r
			}
			return -1
		}, data)

		_, err2 := f.WriteString(clean + "\n")

		if err2 != nil {
			log.Fatal(err2)
		}

		webhookPrivnote(clean, s.State.User, guild.Name, channel.Name, cryptData)
		logWithTime("<yellow>[-] Wrote the content of the privnote to privnotes.txt</>")
	}
}
