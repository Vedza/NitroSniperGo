> # NitroSniperGo

[![GitHub release](https://img.shields.io/github/release/Vedzaa/NitroSniperGo.svg?style=flat)](https://github.com/Vedzaa/NitroSniperGo/releases)
[![GitHub All Releases](https://img.shields.io/github/downloads/vedza/NitroSniperGo/total?style=flat)](https://github.com/vedza/NitroSniperGo/releases)
[![Views](https://hits.seeyoufarm.com/api/count/incr/badge.svg?url=https://github.com/Vedza/NitroSniperGo&title=Views)](https://github.com/Vedza/NitroSniperGo)                    

<a href="https://www.buymeacoffee.com/Vedza" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/default-orange.png" alt="Buy Me A Coffee" height="32" width="140"></a>
[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/Vedza/NitroSniperGo/tree/heroku)

Discord Nitro sniper and Giveaway joiner in Go.

![Screenshot](screenshot.png)

#### Features 
* Multiple accounts support to claim on one main account
* Optional Counter for max Nitro activations
* Optional main account sniper to only claim code from alts
* Cooldown for # hour(s) after redeeming # nitro code(s)
* Duplicate code detection
* Optional Giveaway joiner and only Nitro Giveaway joiner
* DM host with custom message if giveaway won
* Optional Privnote sniper
* Optional custom status
* Optional Invite link sniper
* Optional Counter for max server joined
* Cooldown for # hour(s) after joining # server(s)
* Webhook support with good only mode that report only codes applied and giveaways won.
* Blacklist servers to not join any giveaways on these servers
* Custom delay to join giveaways, servers and DM giveaways host

#### Usage

Edit `settings.json`
``` json5
{
  "main_token": "Nz...", // Your main token here
  "main_sniper" : true, // Enable or not Nitro sniper on main account (It will only claim code from alts)
  "alts_tokens": [ // Alts token
    "",  // Token1
    ""  // Token2
  ],
  "nitro_max": 2, // Max Nitro before cooldown
  "cooldown": 24, // in Hour
  "main_status": "", // online, offline, idle, dnd, invisible
  "alts_status": "", // online, offline, idle, dnd, invisible
  "giveaway_sniper": true // Enable or not giveaway joiner
  "nitro_giveaway_sniper": true, // Only join Nitro gieaways
  "giveaway_delay": 2, // Delay in second before joining giveaway
  "giveaway_dm": "Hey, I won a giveaway !", // DM sent to giveaway host, leave empty to not send any dm
  "giveaway_dm_delay": 10, // Delay in second before sending DM
  "privnote_sniper": true, // Enable or not Privnote sniper
  "invite_sniper": true, // Enable or not server invite sniper
  "invite_delay": {
    "min": 5, // Minimum delay in minute before joining server
    "max": 10 // Maximum delay in minute before joining server
  },
  "invite_max" : 1,  // Max Servers joined before cooldown
  "invite_cooldown" : 6, // in Hour
  "webhook": {
    "url": "",
    "good_only": true // Will trigger webhook only when you applied a Nitro code or won a giveaway
  },
  "blacklist_servers": [
    "727880228696457325",
    "727888218646457612"
  ] // IDs of servers you don't want the giveaway joiner to work on
}
```

You have multiple choices to run the sniper : 

- [Deploy on Heroku](https://heroku.com/deploy?template=https://github.com/Vedza/NitroSniperGo/tree/heroku) (Free 24/7)
   * Deploy
   * Resources -> enable worker
   * See logs in More -> View logs

- Download the latest [release](https://github.com/Vedza/NitroSniperGo/releases)

- Compile it yourself
  ``` sh
  go mod download
  go build
  ./NitroSniperGo
  ```
 
#### How to obtain your token
https://github.com/Tyrrrz/DiscordChatExporter/wiki/Obtaining-Token-and-Channel-IDs#how-to-get-a-user-token

#### Known issues
* `error unmarshalling READY event` is not a problem, it just happens because you're doing a self bot
* It looks like Discord added a security feature where your token change every time but also expire, that might be the reason why the sniper doesn't work after some time or if you get an unauthorized error when sniping Nitro
* Some welcome bots mention giveaways that might cause a false positive
* Privnote sniper makes the program crash sometimes, disable it in settings if that happens to you until I find a solution

#### Disclaimer
This is against TOS and can get your account banned, especially if you run multiple instance at the same time and/or claim too many Nitros in a too short amount of time. Use it at your own risks.

> *If you like my sniper consider putting a star on this repo !*
