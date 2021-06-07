#!/bin/bash
git checkout master
rm -rf NitroSniperGo_build_*
mkdir NitroSniperGo_build_win64 NitroSniperGo_build_linux NitroSniperGo_build_mac
env GOOS=windows GOARCH=amd64 go build && cp settings.json settings.json NitroSniperGo.exe NitroSniperGo_build_win64
env GOOS=linux go build && cp settings.json settings.json NitroSniperGo NitroSniperGo_build_linux
go build && cp NitroSniperGo settings.json sound.mp3 NitroSniperGo_build_mac
zip -r NitroSniperGo_build_linux NitroSniperGo_build_linux
zip -r NitroSniperGo_build_win64 NitroSniperGo_build_win64
zip -r NitroSniperGo_build_mac NitroSniperGo_build_mac
rm -rf NitroSniperGo_build_win64 NitroSniperGo_build_linux NitroSniperGo_build_mac
hub release create -d -m "NitroSniperGo Build $1" $1
files=$(find . -type f -name "*.zip" -exec echo '-a' {} \;)
hub release edit $files -m "NitroSniperGo Build $1" $1
rm -rf *.zip NitroSniperGo*
git checkout heroku
git pull
git merge --no-ff master
git push
git checkout replit
git pull
git merge --no-ff heroku
git push
git checkout master
