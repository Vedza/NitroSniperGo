#!/bin/bash

rm -rf NitroSniperGo_build_*
mkdir NitroSniperGo_build_win64 NitroSniperGo_build_linux NitroSniperGo_build_mac
env GOOS=windows GOARCH=amd64 go build && cp settings.json NitroSniperGo.exe NitroSniperGo_build_win64
env GOOS=linux go build && cp settings.json NitroSniperGo NitroSniperGo_build_linux
go build && cp NitroSniperGo settings.json NitroSniperGo_build_mac
zip -r NitroSniperGo_build_linux NitroSniperGo_build_linux
zip -r NitroSniperGo_build_win64 NitroSniperGo_build_win64
zip -r NitroSniperGo_build_mac NitroSniperGo_build_mac
rm -rf NitroSniperGo_build_win64 NitroSniperGo_build_linux NitroSniperGo_build_mac
