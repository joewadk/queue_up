# 03/30/2026
- git init
- project direction defined
- base architecture setup complete
- desktop agent somewhat complete with leetcode redirect
- added logging in `\desktop-agent\logs\enforcement.jsonl`
- added `desktop-run` bash script for simplicity
- modified cooldown for polling up to 30s in `config/config.json`
- modified cooldown to open links to 1 min in `config/config.json`

# 03/31/2026
- added desktop agent exe bundling
- added exe registry config
- added docker and postgres to backend 
- some bash scripts made for docker run and stop
- added aws ec2 infra setup
- added icon to system tray and exe
- updated setup and start scripts 
- added nginx to aws infra for rate limiting
- added rate limiting to nginx configuration for backend