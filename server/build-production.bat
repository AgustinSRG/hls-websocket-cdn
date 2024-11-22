@echo off

call go build -ldflags="-s -w" -o server.exe
