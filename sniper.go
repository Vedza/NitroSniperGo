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
	"github.com/kardianos/osext"
	"github.com/valyala/fasthttp"
	"io/ioutil"
	"log"
	"math/rand"
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
	Tokens struct {
		Main string   `json:"main"`
		Alts []string `json:"alts"`
	} `json:"tokens"`
	Status struct {
		Main string `json:"main"`
		Alts string `json:"alts"`
	} `json:"status"`
	Nitro struct {
		Max        int  `json:"max"`
		Cooldown   int  `json:"cooldown"`
		MainSniper bool `json:"main_sniper"`
		Delay      bool `json:"delay"`
	} `json:"nitro"`
	Giveaway struct {
		Enable           bool     `json:"enable"`
		Delay            int      `json:"delay"`
		DM               string   `json:"dm"`
		DMDelay          int      `json:"dm_delay"`
		BlacklistWords   []string `json:"blacklist_words"`
		WhitelistWords   []string `json:"whitelist_words"`
		BlacklistServers []string `json:"blacklist_servers"`
	} `json:"giveaway"`
	Invite struct {
		Enable bool `json:"enable"`
		Delay  struct {
			Min int `json:"min"`
			Max int `json:"max"`
		} `json:"delay"`
		InviteMax int `json:"max"`
		Cooldown  int `json:"cooldown"`
	} `json:"invite"`
	Privnote struct {
		Enable bool `json:"enable"`
	} `json:"privnote"`
	Webhook struct {
		URL      string `json:"url"`
		GoodOnly bool   `json:"good_only"`
	} `json:"webhook"`
}

type Response struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

var (
	paymentSourceID string
	currentToken    string
	NitroSniped     int
	InviteSniped    int
	SniperRunning   bool
	InviteRunning   bool
	settings        Settings
	nbServers       int
	cache, _        = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,
		MaxCost:     1 << 30,
		BufferItems: 64,
	})
	reGiftLink        = regexp.MustCompile("(discord.com/gifts/|discordapp.com/gifts/|discord.gift/)([a-zA-Z0-9]+)")
	rePrivnote        = regexp.MustCompile("(https://privnote.com/[0-9A-Za-z]+)#([0-9A-Za-z]+)")
	rePrivnoteData    = regexp.MustCompile(`"data": "(.*)",`)
	reInviteServer    = regexp.MustCompile(`"name": "(.*)", "splash"`)
	reGiveaway        = regexp.MustCompile("You won the \\*\\*(.*)\\*\\*")
	reGiveawayMessage = regexp.MustCompile("<https://discordapp.com/channels/(.*)/(.*)/(.*)>")
	rePaymentSourceId = regexp.MustCompile(`("id": ")([0-9]+)"`)
	reInviteLink      = regexp.MustCompile("https://discord.gg/([0-9a-zA-Z]+)")
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

	_, _ = magenta.Print(time.Now().Format("15:04:05 "))
	_, _ = green.Print("[+] " + s.State.User.Username + " joined a new server: ")
	_, _ = yellow.Print(serverName)
	print(" from " + m.Author.String())
	guild, err := s.State.Guild(m.GuildID)
	if err != nil || guild == nil {
		guild, err = s.Guild(m.GuildID)
		if err != nil {
			println()
			if InviteSniped >= settings.Invite.InviteMax {
				InviteRunning = false
				_, _ = magenta.Print(time.Now().Format("15:04:05 "))
				_, _ = yellow.Println("[+] Stopping Invite sniping for now")
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
				_, _ = magenta.Print(time.Now().Format("15:04:05 "))
				_, _ = yellow.Println("[+] Stopping Invite sniping for now")
				time.AfterFunc(time.Hour*time.Duration(settings.Invite.Cooldown), inviteTimerEnd)
			}
		}
	}
	_, _ = magenta.Println(" [" + guild.Name + " > " + channel.Name + "]")
	if InviteSniped >= settings.Invite.InviteMax {
		InviteRunning = false
		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = yellow.Println("[+] Stopping Invite sniping for now")
		time.AfterFunc(time.Hour*time.Duration(settings.Invite.Cooldown), inviteTimerEnd)
	}
}

func join(code string, s *discordgo.Session, m *discordgo.MessageCreate) func() {
	return func() {
		joinServer(code, s, m)
	}
}

func webhookNitro(code string, user *discordgo.User, guild string, channel string, status int, response string) {
	if settings.Webhook.URL == "" || (status <= 0 && settings.Webhook.GoodOnly) {
		return
	}
	var image = "https://i.redd.it/mvoen8wq3w831.png"
	var color = "65290"

	if status == 0 {
		color = "16769024"
		image = ""
	} else if status == -1 {
		image = ""
		color = "16742912"
	}
	body := `
	{
	  "content": null,
	  "embeds": [
		{
		  "color": ` + color + `,
		  "fields": [
			{
			  "name": "Code",
			  "value": "` + code + `",
			  "inline": false
			},
			{
			  "name": "Guild",
			  "value": "` + guild + `",
			  "inline": true
			},
			{
			  "name": "Channel",
			  "value": "` + channel + `",
			  "inline": true
			},
			{
			  "name": "Response",
			  "value": "` + response + `",
			  "inline": false
			}
		  ],
		  "author": {
			"name": "Nitro Sniped !"
		  },
		  "footer": {
			"text": "NitroSniperGo made by Vedza"
		  },
		  "thumbnail": {
			"url": "` + image + `"
		  }
		}
	  ],
	"username": "` + user.Username + `",
  	"avatar_url": "` + user.AvatarURL("") + `"
	}
	`

	req := fasthttp.AcquireRequest()
	req.Header.SetContentType("application/json")
	req.SetBody([]byte(body))
	req.Header.SetMethodBytes([]byte("POST"))
	req.SetRequestURIBytes([]byte(settings.Webhook.URL))
	res := fasthttp.AcquireResponse()

	if err := fasthttp.Do(req, res); err != nil {
		return
	}

	fasthttp.ReleaseRequest(req)
	fasthttp.ReleaseResponse(res)
}

func webhookGiveaway(prize string, user *discordgo.User, guild string, channel string) {
	if settings.Webhook.URL == "" {
		return
	}
	var color = "65290"

	if prize != "" {
		prize = `
			{
			  "name": "Prize",
			  "value": "` + prize + `",
			  "inline": false
			},`
	}

	body := `
	{
	  "content": null,
	  "embeds": [
		{
		  "color": ` + color + `,
		  "fields": [
			` + prize + `
			{
			  "name": "Guild",
			  "value": "` + guild + `",
			  "inline": true
			},
			{
			  "name": "Channel",
			  "value": "` + channel + `",
			  "inline": true
			}
		  ],
		  "author": {
			"name": "Giveaway Won !"
		  },
		  "footer": {
			"text": "NitroSniperGo made by Vedza"
		  },
		  "thumbnail": {
        	"url": "https://media.hearthpwn.com/attachments/96/923/tadapopper.png"
		  }
		}
	  ],
	"username": "` + user.Username + `",
  	"avatar_url": "` + user.AvatarURL("") + `"
	}
	`

	req := fasthttp.AcquireRequest()
	req.Header.SetContentType("application/json")
	req.SetBody([]byte(body))
	req.Header.SetMethodBytes([]byte("POST"))
	req.SetRequestURIBytes([]byte(settings.Webhook.URL))
	res := fasthttp.AcquireResponse()

	if err := fasthttp.Do(req, res); err != nil {
		return
	}

	fasthttp.ReleaseRequest(req)
	fasthttp.ReleaseResponse(res)
}

func webhookPrivnote(content string, user *discordgo.User, guild string, channel string, data string) {
	if settings.Webhook.URL == "" {
		return
	}
	var color = "65290"

	content = "`" + content + "`"
	data = "`" + data + "`"
	body := `
	{
	  "content": null,
	  "embeds": [
		{
		  "color": ` + color + `,
		  "fields": [
			{
			  "name": "Guild",
			  "value": "` + guild + `",
			  "inline": true
			},
			{
			  "name": "Channel",
			  "value": "` + channel + `",
			  "inline": true
			},
 			{
          	"name": "Content",
          	"value": "` + content + `"
        	},
			{
          	"name": "Encrypted",
          	"value": "` + data + `"
        	}
		  ],
		  "author": {
			"name": "Privnote Sniped !"
		  },
		  "footer": {
			"text": "NitroSniperGo made by Vedza"
		  },
		  "thumbnail": {
        	"url": "https://images.emojiterra.com/twitter/512px/1f4cb.png"
		  }
		}
	  ],
	"username": "` + user.Username + `",
  	"avatar_url": "` + user.AvatarURL("") + `"
	}
	`

	req := fasthttp.AcquireRequest()
	req.Header.SetContentType("application/json")
	req.SetBody([]byte(body))
	req.Header.SetMethodBytes([]byte("POST"))
	req.SetRequestURIBytes([]byte(settings.Webhook.URL))
	res := fasthttp.AcquireResponse()

	if err := fasthttp.Do(req, res); err != nil {
		return
	}

	fasthttp.ReleaseRequest(req)
	fasthttp.ReleaseResponse(res)
}

func getPaymentSourceId() {
	var strRequestURI = []byte("https://discord.com/api/v8/users/@me/billing/payment-sources")
	req := fasthttp.AcquireRequest()
	req.Header.Set("authorization", settings.Tokens.Main)
	req.Header.SetMethodBytes([]byte("GET"))
	req.SetRequestURIBytes(strRequestURI)
	res := fasthttp.AcquireResponse()

	if err := fasthttp.Do(req, res); err != nil {
		return
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
	executablePath, err := osext.ExecutableFolder()
	if err != nil {
		log.Fatal("Error: Couldn't determine working directory: " + err.Error())
	}
	os.Chdir(executablePath)
	file, err := ioutil.ReadFile("settings.json")
	if err != nil {
		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = red.Println("[x] Failed read file: ", err)
		time.Sleep(4 * time.Second)
		os.Exit(-1)
	}

	err = json.Unmarshal(file, &settings)
	if err != nil {
		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = red.Println("[x] Failed to parse JSON file: ", err)
		time.Sleep(4 * time.Second)
		os.Exit(-1)
	}

	NitroSniped = 0
	InviteSniped = 0
	SniperRunning = true
	InviteRunning = true
}
func timerEnd() {
	SniperRunning = true
	NitroSniped = 0
	_, _ = magenta.Print(time.Now().Format("15:04:05 "))
	_, _ = green.Println("[+] Starting Nitro sniping")
}

func inviteTimerEnd() {
	InviteSniped = 0
	InviteRunning = true
	_, _ = magenta.Print(time.Now().Format("15:04:05 "))
	_, _ = green.Println("[+] Starting Nitro sniping")
}

func run(token string, finished chan bool, index int) {
	currentToken = token
	dg, err := discordgo.New(token)
	if err != nil {
		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = red.Println("[x] Error creating Discord session for "+token+",", err)
		time.Sleep(4 * time.Second)
		os.Exit(-1)
	} else {
		err = dg.Open()
		if err != nil {
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = red.Println("[x] Error opening connection for "+token+",", err)
			time.Sleep(4 * time.Second)
			os.Exit(-1)
		} else {
			nbServers += len(dg.State.Guilds)
			dg.AddHandler(messageCreate)
			if settings.Status.Alts != "" {
				_, _ = dg.UserUpdateStatus(discordgo.Status(settings.Status.Alts))
			}
		}
	}
	if index == len(settings.Tokens.Alts)-1 {
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

	if settings.Tokens.Main == "" {
		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = red.Println("[x] You must put your token in settings.json")
		time.Sleep(4 * time.Second)
		os.Exit(-1)
	}

	finished := make(chan bool)

	settings.Tokens.Alts = deleteEmpty(settings.Tokens.Alts)

	if len(settings.Tokens.Alts) != 0 {
		for i, token := range settings.Tokens.Alts {
			go run(token, finished, i)
		}
	}

	var dg *discordgo.Session
	var err error

	if settings.Nitro.MainSniper {
		dg, err = discordgo.New(settings.Tokens.Main)

		if err != nil {
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = red.Println("[x] Error creating Discord session for "+settings.Tokens.Main+",", err)
			time.Sleep(4 * time.Second)
			os.Exit(-1)
		}

		err = dg.Open()
		if err != nil {
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = red.Println("[x] Error opening connection for "+settings.Tokens.Main+",", err)
			time.Sleep(4 * time.Second)
			os.Exit(-1)
		}

		dg.AddHandler(messageCreate)

		if settings.Status.Main != "" {
			_, _ = dg.UserUpdateStatus(discordgo.Status(settings.Status.Main))
		}

		nbServers += len(dg.State.Guilds)
	}

	if len(settings.Tokens.Alts) != 0 {
		<-finished
	}

	getPaymentSourceId()

	t := time.Now()
	_, _ = cyan.Print("Sniping Discord Nitro")
	if settings.Giveaway.Enable == true && settings.Privnote.Enable == false {
		_, _ = cyan.Print(" and Giveaway")
	} else if settings.Giveaway.Enable == true && settings.Privnote.Enable == true {
		_, _ = cyan.Print(", Giveaway and Privnote")
	} else if settings.Privnote.Enable == true {
		_, _ = cyan.Print(" and Privnote")
	}
	if settings.Nitro.MainSniper {
		_, _ = cyan.Print(" for " + dg.State.User.Username + " on " + strconv.Itoa(nbServers) + " servers and " + strconv.Itoa(len(settings.Tokens.Alts)+1) + " accounts 🔫\n\n")
	} else {
		_, _ = cyan.Print(" on " + strconv.Itoa(nbServers) + " servers and " + strconv.Itoa(len(settings.Tokens.Alts)) + " accounts 🔫\n\n")
	}
	_, _ = magenta.Print(t.Format("15:04:05 "))
	fmt.Println("[+] Sniper is ready")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	if settings.Nitro.MainSniper {
		_ = dg.Close()
	}
}

func checkCode(bodyString string, code string, user *discordgo.User, guild string, channel string, diff time.Duration) {

	var response Response
	err := json.Unmarshal([]byte(bodyString), &response)

	if err != nil {
		return
	}
	_, _ = magenta.Print(time.Now().Format("15:04:05 "))
	if strings.Contains(bodyString, "redeemed") {
		_, _ = yellow.Print("[-] " + response.Message)
		if settings.Nitro.Delay {
			println(" Delay: " + strconv.FormatInt(int64(diff/time.Millisecond), 10) + "ms")
		} else {
			println()
		}
		webhookNitro(code, user, guild, channel, 0, response.Message)
	} else if strings.Contains(bodyString, "nitro") {
		println(bodyString)
		_, _ = green.Print("[+] Nitro applied !")
		if settings.Nitro.Delay {
			println(" Delay: " + strconv.FormatInt(int64(diff/time.Millisecond), 10) + "ms")
		} else {
			println()
		}
		webhookNitro(code, user, guild, channel, 1, "Nitro applied ")
		NitroSniped++
		if NitroSniped >= settings.Nitro.Max {
			SniperRunning = false
			time.AfterFunc(time.Hour*time.Duration(settings.Nitro.Cooldown), timerEnd)
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = yellow.Println("[+] Stopping Nitro sniping for now")
		}
	} else if strings.Contains(bodyString, "Unknown Gift Code") {
		_, _ = red.Print("[x] " + response.Message)
		if settings.Nitro.Delay {
			println(" Delay: " + strconv.FormatInt(int64(diff/time.Millisecond), 10) + "ms")
		} else {
			println()
		}
	} else {
		_, _ = yellow.Print("[?] " + response.Message)
		if settings.Nitro.Delay {
			println(" Delay: " + strconv.FormatInt(int64(diff/time.Millisecond), 10) + "ms")
		} else {
			println()
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

	_, _ = magenta.Print(time.Now().Format("15:04:05 "))
	_, _ = green.Print("[-] " + s.State.User.Username + " sniped code: ")
	_, _ = red.Print(code[2])
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

	print(" from " + m.Author.String())
	_, _ = magenta.Println(" [" + guild.Name + " > " + channel.Name + "]")
	checkCode(bodyString, code[2], s.State.User, guild.Name, channel.Name, diff)
}

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

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	if reGiftLink.Match([]byte(m.Content)) && SniperRunning {
		checkGiftLink(s, m, m.Content, time.Now())
	} else if settings.Giveaway.Enable && !contains(settings.Giveaway.BlacklistServers, m.GuildID) && (strings.Contains(strings.ToLower(m.Content), "**giveaway**") || (strings.Contains(strings.ToLower(m.Content), "react with") && strings.Contains(strings.ToLower(m.Content), "giveaway"))) && m.Author.Bot {
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
		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = yellow.Print("[-] " + s.State.User.Username + " entered a Giveaway")
		_, _ = magenta.Println(" [" + guild.Name + " > " + channel.Name + "]")
		_ = s.MessageReactionAdd(m.ChannelID, m.ID, "🎉")

	} else if (strings.Contains(strings.ToLower(m.Content), "giveaway") || strings.Contains(strings.ToLower(m.Content), "win") || strings.Contains(strings.ToLower(m.Content), "won")) && strings.Contains(m.Content, s.State.User.ID) && m.Author.Bot {
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
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = green.Print("[+] " + s.State.User.Username + " Won Giveaway")
			if len(won) > 1 {
				_, _ = green.Print(": ")
				_, _ = cyan.Println(won[1])
				webhookGiveaway(won[1], s.State.User, guild.Name, channel.Name)
			}
			webhookGiveaway("", s.State.User, guild.Name, channel.Name)
			_, _ = magenta.Println(" [" + guild.Name + " > " + channel.Name + "]")
		} else {
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = green.Print("[+] " + s.State.User.Username + " Won Giveaway")
			if len(won) > 1 {
				_, _ = green.Print(": ")
				webhookGiveaway(won[1], s.State.User, guild.Name, channel.Name)
				_, _ = cyan.Print(won[1])
			} else {
				webhookGiveaway("", s.State.User, guild.Name, channel.Name)
			}
			_, _ = magenta.Println(" [" + guild.Name + " > " + channel.Name + "]")
		}

		if settings.Giveaway.DM != "" {
			var giveawayHost = findHost(s, m)
			if giveawayHost == "" {
				_, _ = magenta.Print(time.Now().Format("15:04:05 "))
				_, _ = red.Print("[x] Couldn't determine giveaway host")
				_, _ = magenta.Println(" [" + guild.Name + " > " + channel.Name + "]")
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
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = green.Print("[+] " + s.State.User.Username + " sent DM to host: ")
			_, _ = fmt.Println(host.Username + "#" + host.Discriminator)
		}
	} else if rePrivnote.Match([]byte(m.Content)) && settings.Privnote.Enable {
		var link = rePrivnote.FindStringSubmatch(m.Content)
		var strRequestURI = link[1]
		var password = link[2]

		_, _ = magenta.Print(time.Now().Format("15:04:05 "))
		_, _ = green.Print("[-] " + s.State.User.Username + " sniped PrivNote: " + rePrivnote.FindStringSubmatch(m.Content)[0])

		print(" from " + m.Author.String())

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
		_, _ = magenta.Println(" [" + guild.Name + " > " + channel.Name + "]")

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
		if reGiftLink.Match([]byte(data)) && SniperRunning {
			code := reGiftLink.FindStringSubmatch(data)
			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			_, _ = green.Print("[+] Found a gift link in it: ")
			_, _ = red.Println(code[2])
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

			_, _ = magenta.Print(time.Now().Format("15:04:05 "))
			webhookPrivnote(clean, s.State.User, guild.Name, channel.Name, cryptData)
			_, _ = yellow.Print("[-] Wrote the content of the privnote to privnotes.txt")
		}
	} else if reInviteLink.Match([]byte(m.Content)) && settings.Invite.Enable {

		if s.Token == settings.Tokens.Main || !InviteRunning {
			return
		}
		code := reInviteLink.FindStringSubmatch(m.Content)[1]
		var f = join(code, s, m)
		n := rand.Intn(settings.Invite.Delay.Max - settings.Invite.Delay.Min)
		time.AfterFunc(time.Minute*(time.Duration(settings.Invite.Delay.Min)+time.Duration(n)), f)
	}
}
