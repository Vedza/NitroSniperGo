package main

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/fatih/color"
	"github.com/valyala/fasthttp"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	strconv "strconv"
	"strings"
	"syscall"
	"time"
)

type Settings struct {
	Token               string   `json:"token"`
	NitroMax            int      `json:"nitro_max"`
	Cooldown            int      `json:"cooldown"`
	GiveawaySniper      bool     `json:"giveaway_sniper"`
	NitroGiveawaySniper bool     `json:"nitro_giveaway_sniper"`
	GiveawayDm          string   `json:"giveaway_dm"`
	Webhook             string   `json:"webhook"`
	BlacklistServers    []string `json:"blacklist_servers"`
}

type Response struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

var (
	userID            string
	paymentSourceID   string
	NitroSniped       int
	SniperRunning     bool
	settings          Settings
	re                = regexp.MustCompile("(discord.com/gifts/|discordapp.com/gifts/|discord.gift/)([a-zA-Z0-9]+)")
	_                 = regexp.MustCompile("https://privnote.com/.*")
	reGiveaway        = regexp.MustCompile("You won the \\*\\*(.*)\\*\\*")
	reGiveawayMessage = regexp.MustCompile("<https://discordapp.com/channels/(.*)/(.*)/(.*)>")
	rePaymentSourceId = regexp.MustCompile(`("id": ")([0-9]+)"`)
	magenta           = color.New(color.FgMagenta)
	green             = color.New(color.FgGreen)
	yellow            = color.New(color.FgYellow)
	red               = color.New(color.FgRed)
	cyan              = color.New(color.FgCyan)
)

func contains(array []string, value string) bool {
	for _, v := range array {
		if v == value {
			return true
		}
	}

	return false
}

func webhook(code string, response string, sender string, color string) {
	if settings.Webhook == "" {
		return
	}
	var body = `{
		"content": null,
		"embeds": [
	{
		"title": "Nitro Sniped",
		"description": "**` + code + `**\n*` + sender + `*\n` + response + `",
		"color": ` + color + `,
		"footer": {
			"text": "Don't forget to leave a star on my GitHub repo !"
		}
	}
],
	"username": "NitroSniper",
	"avatar_url": "https://avatars0.githubusercontent.com/u/28839427"
	}`
	req := fasthttp.AcquireRequest()
	req.Header.SetContentType("application/json")
	req.SetBody([]byte(body))
	req.Header.SetMethodBytes([]byte("POST"))
	req.SetRequestURIBytes([]byte(settings.Webhook))
	res := fasthttp.AcquireResponse()

	if err := fasthttp.Do(req, res); err != nil {
		panic("handle error")
	}

	fasthttp.ReleaseRequest(req)
	fasthttp.ReleaseResponse(res)

}
func getPaymentSourceId() {
	var strRequestURI = []byte("https://discord.com/api/v8/users/@me/billing/payment-sources")
	req := fasthttp.AcquireRequest()
	req.Header.Set("authorization", settings.Token)
	req.Header.SetMethodBytes([]byte("GET"))
	req.SetRequestURIBytes(strRequestURI)
	res := fasthttp.AcquireResponse()

	if err := fasthttp.Do(req, res); err != nil {
		panic("handle error")
	}

	fasthttp.ReleaseRequest(req)

	body := res.Body()

	id := rePaymentSourceId.FindStringSubmatch(string(body))

	if id == nil {
		paymentSourceID = "null"
	}
	if len(id) > 1 {
		paymentSourceID = id[2]
	}
}
func init() {
	file, err := ioutil.ReadFile("settings.json")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed read file: %s\n", err)
		os.Exit(1)
	}

	err = json.Unmarshal(file, &settings)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to parse JSON file: %s\n", err)
		os.Exit(1)
	}
	NitroSniped = 0
	SniperRunning = true
}
func timerEnd() {
	SniperRunning = true
	NitroSniped = 0
	_, _ = magenta.Print(time.Now().Format("15:04:05 "))
	_, _ = green.Println("[+] Starting Nitro sniping")
}

func main() {
	dg, err := discordgo.New(settings.Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(messageCreate)

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	c := exec.Command("clear")

	c.Stdout = os.Stdout
	_ = c.Run()
	color.Red(`
▓█████▄  ██▓  ██████  ▄████▄   ▒█████   ██▀███  ▓█████▄      ██████  ███▄    █  ██▓ ██▓███  ▓█████  ██▀███
▒██▀ ██▌▓██▒▒██    ▒ ▒██▀ ▀█  ▒██▒  ██▒▓██ ▒ ██▒▒██▀ ██▌   ▒██    ▒  ██ ▀█   █ ▓██▒▓██░  ██▒▓█   ▀ ▓██ ▒ ██▒
░██   █▌▒██▒░ ▓██▄   ▒▓█    ▄ ▒██░  ██▒▓██ ░▄█ ▒░██   █▌   ░ ▓██▄   ▓██  ▀█ ██▒▒██▒▓██░ ██▓▒▒███   ▓██ ░▄█ ▒
░▓█▄   ▌░██░  ▒   ██▒▒▓▓▄ ▄██▒▒██   ██░▒██▀▀█▄  ░▓█▄   ▌     ▒   ██▒▓██▒  ▐▌██▒░██░▒██▄█▓▒ ▒▒▓█  ▄ ▒██▀▀█▄
░▒████▓ ░██░▒██████▒▒▒ ▓███▀ ░░ ████▓▒░░██▓ ▒██▒░▒████▓    ▒██████▒▒▒██░   ▓██░░██░▒██▒ ░  ░░▒████▒░██▓ ▒██▒
▒▒▓  ▒ ░▓  ▒ ▒▓▒ ▒ ░░ ░▒ ▒  ░░ ▒░▒░▒░ ░ ▒▓ ░▒▓░ ▒▒▓  ▒    ▒ ▒▓▒ ▒ ░░ ▒░   ▒ ▒ ░▓  ▒▓▒░ ░  ░░░ ▒░ ░░ ▒▓ ░▒▓░
░ ▒  ▒  ▒ ░░ ░▒  ░ ░  ░  ▒     ░ ▒ ▒░   ░▒ ░ ▒░ ░ ▒  ▒    ░ ░▒  ░ ░░ ░░   ░ ▒░ ▒ ░░▒ ░      ░ ░  ░  ░▒ ░ ▒░
░ ░  ░  ▒ ░░  ░  ░  ░        ░ ░ ░ ▒    ░░   ░  ░ ░  ░    ░  ░  ░     ░   ░ ░  ▒ ░░░          ░     ░░   ░
░     ░        ░  ░ ░          ░ ░     ░        ░             ░           ░  ░              ░  ░   ░
░                   ░                           ░
	`)

	getPaymentSourceId()

	t := time.Now()
	color.Cyan("Sniping Discord Nitro and Giveaway on " + strconv.Itoa(len(dg.State.Guilds)) + " Servers 🔫\n\n")

	_, _ = magenta.Print(t.Format("15:04:05 "))
	fmt.Println("[+] Bot is ready")
	userID = dg.State.User.ID

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	_ = dg.Close()
}

func checkCode(bodyString string, code string, sender string) {

	var response Response

	err := json.Unmarshal([]byte(bodyString), &response)

	if err != nil {
		return
	}
	_, _ = magenta.Print(time.Now().Format("15:04:05 "))
	if strings.Contains(bodyString, "redeemed") {
		color.Yellow("[-] " + response.Message)
		webhook(code, response.Message, sender, "16774415")
	} else if strings.Contains(bodyString, "nitro") {
		_, _ = green.Println("[+] " + response.Message)
		webhook(code, response.Message, sender, "2948879")
		NitroSniped++
		if NitroSniped == settings.NitroMax {
			SniperRunning = false
			time.AfterFunc(time.Hour*time.Duration(settings.Cooldown), timerEnd)
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = yellow.Println("[+] Stopping Nitro sniping for now")

		}
	} else if strings.Contains(bodyString, "Unknown Gift Code") {
		_, _ = red.Println("[x] " + response.Message)
		webhook(code, response.Message, sender, "16715535")

	} else {
		color.Yellow("[?] " + response.Message)
		webhook(code, response.Message, sender, "16744975")
	}

}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	if re.Match([]byte(m.Content)) && SniperRunning {

		code := re.FindStringSubmatch(m.Content)

		if len(code) < 2 {
			return
		}

		if len(code[2]) < 16 {
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = red.Print("[=] Auto-detected a fake code: ")
			_, _ = red.Print(code[2])
			fmt.Println(" from " + m.Author.String())
			return
		}
		var strRequestURI = []byte("https://discordapp.com/api/v8/entitlements/gift-codes/" + code[2] + "/redeem")
		req := fasthttp.AcquireRequest()
		req.Header.SetContentType("application/json")
		req.Header.Set("authorization", settings.Token)
		req.SetBody([]byte(`{"channel_id":` + m.ChannelID + `,"payment_source_id": ` + paymentSourceID + `}`))
		req.Header.SetMethodBytes([]byte("POST"))
		req.SetRequestURIBytes(strRequestURI)
		res := fasthttp.AcquireResponse()

		if err := fasthttp.Do(req, res); err != nil {
			panic("handle error")
		}

		fasthttp.ReleaseRequest(req)

		body := res.Body()

		bodyString := string(body)
		fasthttp.ReleaseResponse(res)

		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = green.Print("[-] Sniped code: ")
		_, _ = red.Print(code[2])
		guild, err := s.State.Guild(m.GuildID)
		if err != nil || guild == nil {
			guild, err = s.Guild(m.GuildID)
			if err != nil {
				println()
				checkCode(bodyString, code[2], "")
				return
			}
		}

		channel, err := s.State.Channel(m.ChannelID)
		if err != nil || guild == nil {
			channel, err = s.Channel(m.ChannelID)
			if err != nil {
				println()
				checkCode(bodyString, code[2], guild.Name+" > "+channel.Name)
				return
			}
		}

		print(" from " + m.Author.String())
		_, _ = magenta.Println(" [" + guild.Name + " > " + channel.Name + "]")
		checkCode(bodyString, code[2], guild.Name+" > "+channel.Name)

	} else if settings.GiveawaySniper && !contains(settings.BlacklistServers, m.GuildID) && (strings.Contains(strings.ToLower(m.Content), "**giveaway**") || (strings.Contains(strings.ToLower(m.Content), "react with") && strings.Contains(strings.ToLower(m.Content), "giveaway"))) {
		if settings.NitroGiveawaySniper {
			if len(m.Embeds) > 0 && m.Embeds[0].Author != nil {
				if !strings.Contains(strings.ToLower(m.Embeds[0].Author.Name), "nitro") {
					return
				}
			} else {
				return
			}
		}
		time.Sleep(time.Minute)
		guild, err := s.State.Guild(m.GuildID)
		if err != nil || guild == nil {
			guild, err = s.Guild(m.GuildID)
			if err != nil {
				return
			}
		}

		channel, err := s.State.Channel(m.ChannelID)
		if err != nil || guild == nil {
			channel, err = s.Channel(m.ChannelID)
			if err != nil {
				return
			}
		}
		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = yellow.Print("[-] Enter Giveaway ")
		_, _ = magenta.Println(" [" + guild.Name + " > " + channel.Name + "]")
		_ = s.MessageReactionAdd(m.ChannelID, m.ID, "🎉")

	} else if (strings.Contains(strings.ToLower(m.Content), "giveaway") || strings.Contains(strings.ToLower(m.Content), "win") || strings.Contains(strings.ToLower(m.Content), "won")) && strings.Contains(m.Content, userID) {
		reGiveawayHost := regexp.MustCompile("Hosted by: <@(.*)>")
		won := reGiveaway.FindStringSubmatch(m.Content)
		giveawayID := reGiveawayMessage.FindStringSubmatch(m.Content)
		guild, err := s.State.Guild(m.GuildID)
		if err != nil || guild == nil {
			guild, err = s.Guild(m.GuildID)
			if err != nil {
				return
			}
		}

		channel, err := s.State.Channel(m.ChannelID)
		if err != nil || guild == nil {
			channel, err = s.Channel(m.ChannelID)
			if err != nil {
				return
			}
		}
		if giveawayID == nil {
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = green.Print("[+] Won Giveaway")
			if len(won) > 1 {
				_, _ = green.Print(": ")
				_, _ = cyan.Println(won[1])
			}
			_, _ = magenta.Println(" [" + guild.Name + " > " + channel.Name + "]")

			return
		}
		messages, _ := s.ChannelMessages(m.ChannelID, 1, "", "", giveawayID[3])

		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = green.Print("[+] Won Giveaway")
		if len(won) > 1 {
			_, _ = green.Print(": ")
			_, _ = cyan.Print(won[1])
		}
		_, _ = magenta.Println(" [" + guild.Name + " > " + channel.Name + "]")

		if settings.GiveawayDm != "" {
			giveawayHost := reGiveawayHost.FindStringSubmatch(messages[0].Embeds[0].Description)
			if len(giveawayHost) < 2 {
				return
			}
			hostChannel, err := s.UserChannelCreate(giveawayHost[1])

			if err != nil {
				return
			}
			time.Sleep(time.Second * 9)

			_, err = s.ChannelMessageSend(hostChannel.ID, settings.GiveawayDm)
			if err != nil {
				return
			}

			host, _ := s.User(giveawayHost[1])
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = green.Print("[+] Sent DM to host: ")
			_, _ = fmt.Println(host.Username + "#" + host.Discriminator)
		}
	}

}
