#!/bin/bash
curl -s -X POST --data-urlencode "payload={\"channel\": \"#alerts\", \"username\": \"my slack username\", \"text\": \"Something is wrong with $1\", \"icon_emoji\": \":bear:\"}" https://hooks.slack.com/[EDIT HERE] &> /dev/null
echo "[$(date)] ✗ health-check of service $1 failed"
