package app

// This file contains extended functionality for the event editor.
//
// As of the current implementation, the Script Manager has been successfully
// integrated into the application with the following features:
//
// 1. Hotkey Integration:
//    - Press 'S' to open the Script Manager modal
//    - Press 'ESC' or mouse back button to close it
//
// 2. Script Management:
//    - Lists all .js files in the 'scripts' directory
//    - Execute button to run scripts using the JavaScript engine
//    - Open Externally button to open scripts in default editor
//    - Terminate functionality for running scripts
//    - Support for multiple concurrent running scripts
//
// 3. Visual Indicators:
//    - Running scripts are highlighted in bright green text
//    - Small green indicator circle shows next to running script names
//    - Execute button turns red and shows "Terminate" for selected running scripts
//    - Normal Execute button for non-running scripts
//
// 4. UI Features:
//    - Scrollable script list
//    - Selection highlighting
//    - Modern modal design matching guild manager style
//    - Responsive to mouse interaction
//    - Per-script state tracking
//
// 5. JavaScript Engine Integration:
//    - Uses javascript.Run() function for persistent scripts
//    - Supports init() and tick() functions for periodic execution
//    - Provides access to eruntime and utils objects
//    - Proper error handling and script lifecycle management
//    - Individual script termination capability
//    - Scripts continue running when manager is closed (background execution)
//    - Reopen manager to view running scripts and terminate if needed
//
// The Script Manager is integrated into the GameplayModule and follows
// the same patterns as other modal managers like GuildManager and LoadoutManager.
// Scripts with init() and tick() functions will run continuously until terminated.
// Scripts continue running in the background even when the Script Manager is closed.
// Users can reopen the Script Manager to view running scripts and terminate them individually.
