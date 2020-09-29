# NitroSniperGo

[![GitHub release](https://img.shields.io/github/release/Vedzaa/NitroSniperGo.svg)](https://github.com/Vedzaa/NitroSniperGo/releases)

Discord Nitro sniper and Giveaway joiner in Go.

It also sends a DM to giveaway host when won.

You can now enable or no Giveaway joiner and setup a cooldown of x hours each x nitros applied.

![Screenshot](screenshot.png)

### Usage

Edit `settings.json`
```
{
  "token": "", // Your token here
  "nitro_max": 2, // Maxi Nitro before cooldown
  "cooldown": 24, // in Hour
  "giveaway_sniper": true // Enable or not giveaway joiner
}

```

```
 go mod download
 go build
 ./NitroSniperGo
 ```
 
### How to obtain your token
https://github.com/Tyrrrz/DiscordChatExporter/wiki/Obtaining-Token-and-Channel-IDs#how-to-get-a-user-token

### Disclaimer
This can get your account banned if you run multiple instance at the same time and/or claim too much Nitros in a too short amount of time. Use it at your own risks.
