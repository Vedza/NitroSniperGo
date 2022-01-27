package main

import (
	"encoding/json"
	"log"
	"os"
	strconv "strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/valyala/fasthttp"
)

func checkCode(bodyString string, code string, user *discordgo.User, guild string, channel string, diff time.Duration) {

	var response Response
	err := json.Unmarshal([]byte(bodyString), &response)

	if err != nil {
		return
	}
	if strings.Contains(bodyString, "redeemed") {
		if settings.Nitro.Delay {
			logWithTime("<yellow>[-] " + response.Message + "</> Delay: " + strconv.FormatInt(int64(diff/time.Millisecond), 10) + "ms")
		} else {
			logWithTime("<yellow>[-] " + response.Message + "</>")
		}
		webhookNitro(code, user, guild, channel, 0, response.Message)
	} else if strings.Contains(bodyString, "nitro") {
		f, err := os.Open("sound.mp3")
		if err != nil {
			log.Fatal(err)
		}

		var format beep.Format
		sound, format, err := mp3.Decode(f)
		if err != nil {
			log.Fatal(err)
		}
		defer sound.Close()

		speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

		done := make(chan bool)
		speaker.Play(beep.Seq(sound, beep.Callback(func() {
			done <- true
		})))

		<-done
		nitroType := ""
		if reNitroType.Match([]byte(bodyString)) {
			nitroType = reNitroType.FindStringSubmatch(bodyString)[1]
		}

		if settings.Nitro.Delay {
			logWithTime("<green>[+] Nitro applied : </><cyan>" + nitroType + "</> Delay:" + strconv.FormatInt(int64(diff/time.Millisecond), 10) + "ms")
		} else {
			logWithTime("<green>[+] Nitro applied : </><cyan>" + nitroType + "</>")
		}
		webhookNitro(code, user, guild, channel, 1, nitroType)
		NitroSniped++
		if NitroSniped >= settings.Nitro.Max {
			SniperRunning = false
			time.AfterFunc(time.Hour*time.Duration(settings.Nitro.Cooldown), timerEnd)
			logWithTime("<yellow>[+] Stopping Nitro sniping for now</>")
		}
	} else if strings.Contains(bodyString, "Unknown Gift Code") {
		if settings.Nitro.Delay {
			logWithTime("<red>[x] " + response.Message + "</> Delay: " + strconv.FormatInt(int64(diff/time.Millisecond), 10) + "ms")
		} else {
			logWithTime("<red>[x] " + response.Message + "</>")
		}
	} else {
		logWithTime("<yellow>[?] " + response.Message + "</>")
		if settings.Nitro.Delay {
			logWithTime("<yellow>[?] " + response.Message + "</> Delay: " + strconv.FormatInt(int64(diff/time.Millisecond), 10) + "ms")
		} else {
			logWithTime("<yellow>[?] " + response.Message + "</>")
		}
		webhookNitro(code, user, guild, channel, -1, response.Message)
	}
	cache.Set(code, "", 1)

}

func checkGiftLink(s *discordgo.Session, m *discordgo.MessageCreate, link string, start time.Time) {

	code := reGiftLink.FindStringSubmatch(link)

	if len(code) < 2 {
		return
	}

	if len(code[2]) < 16 {
		logWithTime("<red>[=] Auto-detected a fake code: " + code[2] + " from " + m.Author.String() + "</>")
		return
	}

	_, found := cache.Get(code[2])
	if found {
		logWithTime("<red>[=] Auto-detected a duplicate code: " + code[2] + " from " + m.Author.String() + "</>")
		return
	}

	var strRequestURI = []byte("https://discordapp.com/api/v8/entitlements/gift-codes/" + code[2] + "/redeem")
	req := fasthttp.AcquireRequest()
	req.Header.SetContentType("application/json")
	req.Header.Set("authorization", settings.Tokens.Main)
	var channelId = "null"
	if s.Token == settings.Tokens.Main {
		channelId = m.ChannelID
	}
	req.SetBody([]byte(`{"channel_id":` + channelId + `,"payment_source_id": ` + paymentSourceID + `}`))
	req.Header.SetMethodBytes([]byte("POST"))
	req.SetRequestURIBytes(strRequestURI)
	res := fasthttp.AcquireResponse()

	if err := fasthttp.Do(req, res); err != nil {
		return
	}
	end := time.Now()
	diff := end.Sub(start)

	fasthttp.ReleaseRequest(req)

	body := res.Body()

	bodyString := string(body)
	fasthttp.ReleaseResponse(res)

	guild, err := s.State.Guild(m.GuildID)
	if err != nil || guild == nil {
		guild, err = s.Guild(m.GuildID)
		if err != nil {
			println()
			checkCode(bodyString, code[2], s.State.User, "DM", m.Author.Username+"#"+m.Author.Discriminator, diff)
			return
		}
	}

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil || guild == nil {
		channel, err = s.Channel(m.ChannelID)
		if err != nil {
			println()
			checkCode(bodyString, code[2], s.State.User, guild.Name, m.Author.Username+"#"+m.Author.Discriminator, diff)
			return
		}
	}

	logWithTime("<green>[-] " + s.State.User.Username + " sniped code: </><red>" + code[2] + "</> from  <magenta>[" + guild.Name + " > " + channel.Name + "]</>")

	checkCode(bodyString, code[2], s.State.User, guild.Name, channel.Name, diff)
}
