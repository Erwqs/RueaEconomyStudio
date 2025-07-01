package app

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
)

// Session represents a game session
type Session struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
	LastPlayed  time.Time `json:"last_played"`
	GameTime    int       `json:"game_time"` // in seconds
	Level       int       `json:"level"`
	Description string    `json:"description"`
}

// SessionManager manages game sessions
type SessionManager struct {
	sessions     []Session
	selectedIdx  int
	mouseX       int
	mouseY       int
	hoveredIdx   int
	onBack       func() // Callback to return to main menu
	sessionsFile string
}

// NewSessionManager creates a new session manager
func NewSessionManager(onBack func()) *SessionManager {
	sm := &SessionManager{
		sessions:     []Session{},
		selectedIdx:  0,
		hoveredIdx:   -1,
		onBack:       onBack,
		sessionsFile: "sessions.json",
	}
	sm.loadSessions()
	return sm
}

// loadSessions loads sessions from file
func (sm *SessionManager) loadSessions() {
	data, err := os.ReadFile(sm.sessionsFile)
	if err != nil {
		// File doesn't exist, start with empty sessions
		return
	}

	err = json.Unmarshal(data, &sm.sessions)
	if err != nil {
		// Invalid JSON, start fresh
		sm.sessions = []Session{}
	}
}

// saveSessions saves sessions to file
func (sm *SessionManager) saveSessions() error {
	data, err := json.Marshal(sm.sessions)
	if err != nil {
		return err
	}
	return os.WriteFile(sm.sessionsFile, data, 0644)
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(name string) {
	session := Session{
		ID:          fmt.Sprintf("session_%d", time.Now().Unix()),
		Name:        name,
		CreatedAt:   time.Now(),
		LastPlayed:  time.Now(),
		GameTime:    0,
		Level:       1,
		Description: "New game session",
	}
	sm.sessions = append(sm.sessions, session)
	sm.saveSessions()
}

// DeleteSession removes a session
func (sm *SessionManager) DeleteSession(index int) {
	if index >= 0 && index < len(sm.sessions) {
		sm.sessions = append(sm.sessions[:index], sm.sessions[index+1:]...)
		if sm.selectedIdx >= len(sm.sessions) && len(sm.sessions) > 0 {
			sm.selectedIdx = len(sm.sessions) - 1
		}
		sm.saveSessions()
	}
}

// Update handles input for session manager
func (sm *SessionManager) Update() {
	// Update mouse position
	sm.mouseX, sm.mouseY = ebiten.CursorPosition()

	// Handle keyboard navigation
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		if len(sm.sessions) > 0 {
			sm.selectedIdx = (sm.selectedIdx - 1 + len(sm.sessions)) % len(sm.sessions)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		if len(sm.sessions) > 0 {
			sm.selectedIdx = (sm.selectedIdx + 1) % len(sm.sessions)
		}
	}

	// Handle action keys
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		if len(sm.sessions) > 0 {
			// Load selected session (placeholder for now)
			// TODO: Implement session loading
		}
	}

	// Handle delete key
	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
		if len(sm.sessions) > 0 {
			sm.DeleteSession(sm.selectedIdx)
		}
	}

	// Handle 'N' key to create new session
	if inpututil.IsKeyJustPressed(ebiten.KeyN) {
		// Create a new session with default name
		newSessionName := fmt.Sprintf("New Session %d", len(sm.sessions)+1)
		sm.CreateSession(newSessionName)
		// Select the newly created session
		sm.selectedIdx = len(sm.sessions) - 1
	}

	// Handle escape to go back
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if sm.onBack != nil {
			sm.onBack()
		}
	}

	// Handle mouse interaction
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if sm.hoveredIdx >= 0 && sm.hoveredIdx < len(sm.sessions) {
			sm.selectedIdx = sm.hoveredIdx
			// Load session
		}
	}
}

// Draw renders the session manager
func (sm *SessionManager) Draw(screen *ebiten.Image) {
	screenW, screenH := screen.Size()
	scaleInfo := getScaleInfo(screenW, screenH)

	// Get scaled font
	scaledFont := loadWynncraftFont(scaleInfo.FontSize)
	fontOffset := getFontVerticalOffset(scaleInfo.FontSize)

	// Draw title
	title := "Session Manager"
	titleBounds := text.BoundString(scaledFont, title)
	titleX := (screenW - titleBounds.Dx()) / 2
	titleY := scaleInfo.scaleInt(100)
	text.Draw(screen, title, scaledFont, titleX, titleY+fontOffset, color.RGBA{255, 255, 255, 255})

	// Reset hovered index
	sm.hoveredIdx = -1

	if len(sm.sessions) == 0 {
		// Draw "Empty" message
		emptyMsg := "No sessions found. Press 'N' to create a new session."
		emptyBounds := text.BoundString(scaledFont, emptyMsg)
		emptyX := (screenW - emptyBounds.Dx()) / 2
		emptyY := scaleInfo.scaleInt(200)
		text.Draw(screen, emptyMsg, scaledFont, emptyX, emptyY+fontOffset, color.RGBA{150, 150, 150, 255})
	} else {
		// Draw sessions
		startY := scaleInfo.scaleInt(150)
		itemHeight := scaleInfo.scaleInt(40)

		for i, session := range sm.sessions {
			y := startY + i*itemHeight
			x := scaleInfo.scaleInt(100)

			// Check mouse hover
			if sm.mouseX >= x && sm.mouseX <= x+scaleInfo.scaleInt(600) &&
				sm.mouseY >= y && sm.mouseY <= y+itemHeight {
				sm.hoveredIdx = i
			}

			// Determine color
			sessionColor := color.RGBA{200, 200, 200, 255}
			if i == sm.selectedIdx || i == sm.hoveredIdx {
				sessionColor = color.RGBA{255, 255, 100, 255}
			}

			// Draw selection indicator
			if i == sm.selectedIdx {
				indicatorX := x - scaleInfo.scaleInt(25)
				text.Draw(screen, "-", scaledFont, indicatorX, y+fontOffset, sessionColor)
			}

			// Format session info
			sessionText := fmt.Sprintf("%s - Level %d - %s",
				session.Name, session.Level, session.LastPlayed.Format("2006-01-02"))
			text.Draw(screen, sessionText, scaledFont, x, y+fontOffset, sessionColor)
		}
	}

	// Draw instructions
	instructions := "Arrow keys/WASD to navigate, Enter to load, Delete to remove, N for new, Escape to go back"
	instrBounds := text.BoundString(scaledFont, instructions)
	instrX := (screenW - instrBounds.Dx()) / 2
	instrY := screenH - scaleInfo.scaleInt(50)
	text.Draw(screen, instructions, scaledFont, instrX, instrY+fontOffset, color.RGBA{150, 150, 150, 255})
}
