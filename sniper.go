package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/dgraph-io/ristretto"
	"github.com/fatih/color"
	"github.com/valyala/fasthttp"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	strconv "strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
)

type Settings struct {
	Maintoken           string   `json:"main_token"`
	AltsTokens          []string `json:"alts_tokens"`
	NitroMax            int      `json:"nitro_max"`
	Cooldown            int      `json:"cooldown"`
	MainStatus          string   `json:"main_status"`
	AltsStatus          string   `json:"alts_status"`
	GiveawaySniper      bool     `json:"giveaway_sniper"`
	PrivnoteSniper      bool     `json:"privnote_sniper"`
	NitroGiveawaySniper bool     `json:"nitro_giveaway_sniper"`
	GiveawayDm          string   `json:"giveaway_dm"`
	Webhook             struct {
		URL      string `json:"url"`
		GoodOnly bool   `json:"good_only"`
	} `json:"webhook"`
	BlacklistServers []string `json:"blacklist_servers"`
}

type Response struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

var (
	paymentSourceID string
	NitroSniped     int
	SniperRunning   bool
	settings        Settings
	nbServers       int
	cache, _        = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,
		MaxCost:     1 << 30,
		BufferItems: 64,
	})
	re                = regexp.MustCompile("(discord.com/gifts/|discordapp.com/gifts/|discord.gift/)([a-zA-Z0-9]+)")
	rePrivnote        = regexp.MustCompile("(https://privnote.com/[0-9A-Za-z]+)#([0-9A-Za-z]+)")
	rePrivnoteData    = regexp.MustCompile(`"data": "(.*)",`)
	reGiveaway        = regexp.MustCompile("You won the \\*\\*(.*)\\*\\*")
	reGiveawayMessage = regexp.MustCompile("<https://discordapp.com/channels/(.*)/(.*)/(.*)>")
	rePaymentSourceId = regexp.MustCompile(`("id": ")([0-9]+)"`)
	magenta           = color.New(color.FgMagenta)
	green             = color.New(color.FgGreen)
	yellow            = color.New(color.FgYellow)
	red               = color.New(color.FgRed)
	cyan              = color.New(color.FgCyan)
)

func Ase256(ciphertext []byte, key string, iv string) string {
	block, err := aes.NewCipher([]byte(key[:]))
	if err != nil {
		log.Fatal(err)
	}

	newtext := make([]byte, len(ciphertext))
	dec := cipher.NewCBCDecrypter(block, []byte(iv))
	dec.CryptBlocks(newtext, ciphertext)
	return string(newtext)
}

func MD5(text string) string {
	hash := md5.Sum([]byte(text))
	return string(hash[:])
}

func openSSLKey(password []byte, salt []byte) (string, string) {
	passSalt := string(password) + string(salt)

	result := MD5(passSalt)

	curHash := MD5(passSalt)
	for i := 0; i < 2; i++ {
		cur := MD5(curHash + passSalt)
		curHash = cur
		result += cur
	}
	return result[0 : 4*8], result[4*8 : 4*8+16]
}

func Base64Decode(message []byte) (b []byte, err error) {
	return base64.RawStdEncoding.DecodeString(string(message))
}

func contains(array []string, value string) bool {
	for _, v := range array {
		if v == value {
			return true
		}
	}

	return false
}

func webhook(title string, code string, response string, sender string, color string) {
	if settings.Webhook.URL == "" || (color != "2948879" && settings.Webhook.GoodOnly == true) {
		return
	}

	if len(sender) > 0 {
		sender = "*" + sender + "*"
	}

	if len(code) > 0 {
		code = "**" + code + "**"
	}
	var body = `{
		"content": null,
		"embeds": [
	{
		"title": "` + title + `",
		"description": "` + code + `\n` + sender + `\n` + response + `",
		"color": ` + color + `
	}
],
	"username": "NitroSniper",
	"avatar_url": "https://avatars0.githubusercontent.com/u/28839427"
	}`
	req := fasthttp.AcquireRequest()
	req.Header.SetContentType("application/json")
	req.SetBody([]byte(body))
	req.Header.SetMethodBytes([]byte("POST"))
	req.SetRequestURIBytes([]byte(settings.Webhook.URL))
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
	req.Header.Set("authorization", settings.Maintoken)
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

func run(token string, finished chan bool, index int) {
	dg, err := discordgo.New(token)
	if err != nil {
		fmt.Println("Error creating Discord session for "+token+" ,", err)
	} else {
		err = dg.Open()
		if err != nil {
			fmt.Println("Error opening connection,", err)
		} else {
			nbServers += len(dg.State.Guilds)
			dg.AddHandler(messageCreate)
			if settings.AltsStatus != "" {
				_, _ = dg.UserUpdateStatus(discordgo.Status(settings.AltsStatus))
			}
		}
	}
	if index == len(settings.AltsTokens)-1 {
		finished <- true
	}
}

func deleteEmpty(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

func main() {
	finished := make(chan bool)

	settings.AltsTokens = deleteEmpty(settings.AltsTokens)

	if len(settings.AltsTokens) != 0 {
		for i, token := range settings.AltsTokens {
			go run(token, finished, i)
		}
	}

	dg, err := discordgo.New(settings.Maintoken)

	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening connection,", err)
		return
	}

	dg.AddHandler(messageCreate)

	if settings.MainStatus != "" {
		_, _ = dg.UserUpdateStatus(discordgo.Status(settings.MainStatus))
	}

	if len(settings.AltsTokens) != 0 {
		<-finished
	}

	nbServers += len(dg.State.Guilds)
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
	_, _ = cyan.Print("Sniping Discord Nitro")
	if settings.GiveawaySniper == true && settings.PrivnoteSniper == false {
		_, _ = cyan.Print(" and Giveaway")
	} else if settings.GiveawaySniper == true && settings.PrivnoteSniper == true {
		_, _ = cyan.Print(", Giveaway and Privnote")
	} else if settings.PrivnoteSniper == true {
		_, _ = cyan.Print(" and Privnote")
	}
	_, _ = cyan.Print(" for " + dg.State.User.Username + " on " + strconv.Itoa(nbServers) + " servers and " + strconv.Itoa(len(settings.AltsTokens)+1) + " accounts 🔫\n\n")

	_, _ = magenta.Print(t.Format("15:04:05 "))
	fmt.Println("[+] Sniper is ready")

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
		webhook("Nitro Sniped", code, response.Message, sender, "16774415")
	} else if strings.Contains(bodyString, "nitro") {
		_, _ = green.Println("[+] " + response.Message)
		webhook("Nitro Sniped", code, response.Message, sender, "2948879")
		NitroSniped++
		if NitroSniped == settings.NitroMax {
			SniperRunning = false
			time.AfterFunc(time.Hour*time.Duration(settings.Cooldown), timerEnd)
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = yellow.Println("[+] Stopping Nitro sniping for now")

		}
	} else if strings.Contains(bodyString, "Unknown Gift Code") {
		_, _ = red.Println("[x] " + response.Message)
		webhook("Nitro Sniped", code, response.Message, sender, "16715535")

	} else {
		color.Yellow("[?] " + response.Message)
		webhook("Nitro Sniped", code, response.Message, sender, "16744975")
	}
	cache.Set(code, "", 1)

}

func checkGiftLink(s *discordgo.Session, m *discordgo.MessageCreate, link string, privnote bool) {

	code := re.FindStringSubmatch(link)

	if len(code) < 2 {
		return
	}

	if privnote == true {
		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = green.Print("[+] Found a gift link in it: ")
		_, _ = red.Println(code[2])
	}

	if len(code[2]) < 16 {
		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = red.Print("[=] Auto-detected a fake code: ")
		_, _ = red.Print(code[2])
		fmt.Println(" from " + m.Author.String())
		return
	}

	_, found := cache.Get(code[2])
	if found {
		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = red.Print("[=] Auto-detected a duplicate code: ")
		_, _ = red.Print(code[2])
		fmt.Println(" from " + m.Author.String())
		return
	}

	var strRequestURI = []byte("https://discordapp.com/api/v8/entitlements/gift-codes/" + code[2] + "/redeem")
	req := fasthttp.AcquireRequest()
	req.Header.SetContentType("application/json")
	req.Header.Set("authorization", settings.Maintoken)
	var channelId = "null"
	if s.Token == settings.Maintoken {
		channelId = m.ChannelID
	}
	req.SetBody([]byte(`{"channel_id":` + channelId + `,"payment_source_id": ` + paymentSourceID + `}`))
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
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	if re.Match([]byte(m.Content)) && SniperRunning {
		checkGiftLink(s, m, m.Content, false)
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

	} else if (strings.Contains(strings.ToLower(m.Content), "giveaway") || strings.Contains(strings.ToLower(m.Content), "win") || strings.Contains(strings.ToLower(m.Content), "won")) && strings.Contains(m.Content, s.State.User.ID) {
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
			_, _ = green.Print("[+] " + s.State.User.Username + " Won Giveaway")
			if len(won) > 1 {
				_, _ = green.Print(": ")
				_, _ = cyan.Println(won[1])
				webhook(s.State.User.Username+" Won Giveaway", won[1], "", guild.Name+" > "+channel.Name, "2948879")
			}
			webhook(s.State.User.Username+" Won Giveaway", "", "", guild.Name+" > "+channel.Name, "2948879")
			_, _ = magenta.Println(" [" + guild.Name + " > " + channel.Name + "]")
			return
		}

		messages, _ := s.ChannelMessages(m.ChannelID, 1, "", "", giveawayID[3])

		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = green.Print("[+] " + s.State.User.Username + " Won Giveaway")
		if len(won) > 1 {
			_, _ = green.Print(": ")
			webhook(s.State.User.Username+" Won Giveaway", won[1], "", guild.Name+" > "+channel.Name, "2948879")
			_, _ = cyan.Print(won[1])
		} else {
			webhook(s.State.User.Username+" Won Giveaway", "", "", guild.Name+" > "+channel.Name, "2948879")
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
	} else if rePrivnote.Match([]byte(m.Content)) && settings.PrivnoteSniper {
		var link = rePrivnote.FindStringSubmatch(m.Content)
		var strRequestURI = link[1]
		var password = link[2]

		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = green.Print("[-] Sniped PrivNote: " + rePrivnote.FindStringSubmatch(m.Content)[0])

		print(" from " + m.Author.String())

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
		_, _ = magenta.Println(" [" + guild.Name + " > " + channel.Name + "]")

		req := fasthttp.AcquireRequest()
		req.Header.SetMethodBytes([]byte("DELETE"))
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.SetRequestURIBytes([]byte(strRequestURI))
		res := fasthttp.AcquireResponse()

		if err := fasthttp.Do(req, res); err != nil {
			panic("handle error")
		}

		fasthttp.ReleaseRequest(req)

		body := res.Body()

		if !rePrivnoteData.Match(body) {
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = red.Println("[x] Privnote already destroyed")
			return
		}
		var cryptData = rePrivnoteData.FindStringSubmatch(string(body))[1]

		var cryptBytes, _ = Base64Decode([]byte(cryptData))

		var salt = cryptBytes[8:16]
		cryptBytes = cryptBytes[16:]

		key, iv := openSSLKey([]byte(password), salt)
		data := Ase256(cryptBytes, key, iv)
		if re.Match([]byte(data)) && SniperRunning {
			checkGiftLink(s, m, data, true)
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

			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			webhook(s.State.User.Username+" Sniped Privnote", clean, "`"+cryptData+"`", guild.Name+" > "+channel.Name, "2948879")
			_, _ = yellow.Print("[-] Wrote the content of the privnote to privnotes.txt")
		}
	}
}
