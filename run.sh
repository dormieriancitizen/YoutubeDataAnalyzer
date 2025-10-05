#!/bin/bash

mkdir -p out data cache
if [ ! -f cache/videoDurationCache.json ]; then
    echo "{}" >cache/videoDurationCache.json
fi

jq -s 'flatten | unique_by(.titleUrl, .time)' ./data/watch-history.json* >watch-history.json
go run main.go --type $1
uv run main.py
