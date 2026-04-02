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

# 04/01/2026
- massive changes to ui, added desktop gui
- using the name provided by the user, we can then use lc's graphql api to do a multitude of things
- allow users to select study path (choose one of the categories from neetcode)
- allow users to see their own submission history in the desktop app
- allow users to submit the lc submission links to the app to mark their project as done (currently wip)
- added java webhook (sanitize and verify lc problems) with go backup 
- dockerized java webhook (submission sanitizer)
- fixing recommmendations for category recommended problems, if neetcode is not sufficient for problems we use the api to seed new problems
- added problem concepts table
- lessened strictness on recommendations (still not perfect)
- added windows exe details 
- manually added nc150 + some extra problems to the pg db

# 04/02/2026
- added killswitch when the user presses "quit" on system tray. all instances of queue up agents are exited
- finally the swapping categories successfully updates the problem queue correctly