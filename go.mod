module NitroSniperGo

go 1.14

require (
	github.com/andersfylling/disgord v0.26.10
	github.com/andersfylling/snowflake/v4 v4.0.2
	github.com/bwmarrin/discordgo v0.23.0
	github.com/dgraph-io/ristretto v0.0.3
	github.com/fatih/color v1.9.0
	github.com/json-iterator/go v1.1.9
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/valyala/fasthttp v1.16.0
	go.uber.org/atomic v1.5.1
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/zap v1.13.0
	golang.org/x/crypto v0.0.0-20191227163750-53104e6ec876
	golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f // indirect
	golang.org/x/net v0.0.0-20200602114024-627f9648deb9
	golang.org/x/tools v0.0.0-20200107181558-a222fb47e2f1 // indirect
	nhooyr.io/websocket v1.7.4
)

replace github.com/andersfylling/disgord => ./disgord@custom
