# WARNING: Half of the codebase is written by claude, read at your own risk

# Ruea Economy Studio - Wynncraft Guild Economy Simulator!

As far as I know, this is the most comprehensive one out there
It aims to closely replicate the economy system in game as much as possible

## First Time Setup
To get things started quick, you want to import all guilds the first time you run the app
- Press G to bring up guild management
- Then press API import to import all guilds and the map

## Features
- Multiple guild supports
- Resource/storage based on how much time has elasped
- Route calculation, affected by tax, border closure and trading style
- Ability to time travel into the future with manual tick advancement or stop the clock with state halting
- Save and load state/session to file for sharing/later use
- Import current map data directly from the API (this only imports guild claims, not their upgrades)
- Ability to enable/disable treasury calculation
- Treasury overrides, allow you to explicitly set a territory to treasury level of your choice.
- Manually editable resource storage at runtime, so your experiment isn't limited by available resource
- Loadouts, apply mode (just like in game) and merge mode
- Tribute system with an option to spawn in or void resource through "Nil Sink/Source" guild instead of from other guild on map
- In-Transit resource inspector, see where all the resources in transit system go!
- Scriptable, write your own JavaScript code to be run within the simulation context

# Keybinds
`G` **Guild management**: you can add/remove guilds or edit guild's claim
`P` **State management**: this menu lets you control the tick rate, modify the logic/calculation/behavior or save and load state session to and from file
`L` **Loadout menu**: create loadout to apply to many territories, there are two application modes: **apply** whice overrides the previous territory setting and apply the loadout's one and **merge** which merge non-zero data from loadout to territory
`B` **Tribute configuration**: set up your tribute here. you can set up a tribute between 2 guilds on the map or spawn in tribute from nothing to the hq (source) or simulate sending out tribute to non-existent guild on the map (sink)
`I` **Resource inspector**: unfinished and abandoned in transit resource inspector and editor
`S` **Script**: run your diy javascript code that will be interacted with the economy simulator here
`Tab` **Territory view switcher**: switch between guild view, resource view, defence and more!

Double click on a territory to open territory menu.
**Note**: if everything shows up as 0 or seems like nothing is working, press P and click Resume (if it says resume) to un-halt the state

Heavily inspired by farog's economy simulator, gsq and avo map and similar projects.
