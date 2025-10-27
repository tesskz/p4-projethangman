package controller

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type Game struct {
	Board         [][]int `json:"board"`
	CurrentPlayer int     `json:"currentPlayer"`
	Status        string  `json:"status"` 
	Winner        int     `json:"winner"`
	LastMove      *Move   `json:"lastMove"`
	Flash         string  `json:"flash"`
}

type Move struct{ Row, Col int }

type Scores struct{ Player1, Player2 int }

type App struct {
	mu          sync.Mutex
	game        *Game
	scores      Scores
	savePath    string
	scoresPath  string
	templateDir string
}

func NewApp(savePath, scoresPath, templateDir string) *App {
	a := &App{savePath: savePath, scoresPath: scoresPath, templateDir: templateDir}
	a.game = newGame()
	// Charger scores si existants
	if sc, err := readJSON[Scores](scoresPath); err == nil {
		a.scores = sc
	} else {
		a.scores = Scores{0, 0}
		_ = writeJSON(scoresPath, a.scores)
	}
	return a
}

func (a *App) Index(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	hasSave := fileExists(a.savePath)
	data := map[string]any{
		"Game":    a.game,
		"HasSave": hasSave,
		"Scores":  a.scores,
	}

	tpl := a.mustParse("index.html")
	if err := tpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	a.game.Flash = ""
}

func (a *App) About(w http.ResponseWriter, r *http.Request) {
	a.render(w, "about.html", nil)
}

func (a *App) Contact(w http.ResponseWriter, r *http.Request) {
	a.render(w, "contact.html", nil)
}

func (a *App) Scoreboard(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	data := map[string]any{"Scores": a.scores}
	a.renderWithData(w, "scoreboard.html", data)
}

func (a *App) NewGame(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.game = newGame()
	_ = os.Remove(a.savePath)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) Resume(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if g, err := readJSON[Game](a.savePath); err == nil {
		a.game = &g
		a.game.Flash = "Partie reprise depuis la sauvegarde."
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) Save(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := writeJSON(a.savePath, a.game); err != nil {
		log.Printf("erreur sauvegarde: %v", err)
	} else {
		a.game.Flash = "Partie sauvegardée."
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) Play(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.game.Status != "ongoing" {
		a.game.Flash = "La partie est terminée. Lance une nouvelle partie."
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	colStr := r.Form.Get("col")
	var col int
	_, err := fmtSscanf(colStr, &col)
	if err != nil || col < 0 || col > 6 {
		a.game.Flash = "Colonne invalide."
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	row, ok := dropDisc(a.game.Board, col, a.game.CurrentPlayer)
	if !ok {
		a.game.Flash = "Colonne pleine. Choisis une autre colonne."
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	a.game.LastMove = &Move{Row: row, Col: col}

	if checkWin(a.game.Board, row, col, a.game.CurrentPlayer) {
		a.game.Status = "win"
		a.game.Winner = a.game.CurrentPlayer
		if a.game.Winner == 1 {
			a.scores.Player1++
		} else {
			a.scores.Player2++
		}
		_ = writeJSON(a.scoresPath, a.scores)
		_ = os.Remove(a.savePath)
		a.game.Flash = "Félicitations, Joueur " + itoa(a.game.Winner) + " !"
	} else if isBoardFull(a.game.Board) {
		a.game.Status = "draw"
		a.game.Flash = "Match nul !"
		_ = os.Remove(a.savePath)
	} else {
		a.game.CurrentPlayer = nextPlayer(a.game.CurrentPlayer)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func newGame() *Game {
	return &Game{Board: createEmptyBoard(), CurrentPlayer: 1, Status: "ongoing"}
}

func createEmptyBoard() [][]int {
	rows, cols := 6, 7
	b := make([][]int, rows)
	for r := 0; r < rows; r++ {
		b[r] = make([]int, cols)
	}
	return b
}

func nextPlayer(p int) int {
	if p == 1 {
		return 2
	}
	return 1
}

func dropDisc(board [][]int, col, player int) (int, bool) {
	if col < 0 || col >= 7 {
		return 0, false
	}
	for r := len(board) - 1; r >= 0; r-- {
		if board[r][col] == 0 {
			board[r][col] = player
			return r, true
		}
	}
	return 0, false
}

func countDirection(board [][]int, row, col, dr, dc, player int) int {
	rows, cols := len(board), len(board[0])
	r, c, cnt := row+dr, col+dc, 0
	for r >= 0 && r < rows && c >= 0 && c < cols && board[r][c] == player {
		cnt++
		r += dr
		c += dc
	}
	return cnt
}

func checkWin(board [][]int, row, col, player int) bool {
	dirs := [][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}
	for _, d := range dirs {
		total := 1 + countDirection(board, row, col, d[0], d[1], player) + countDirection(board, row, col, -d[0], -d[1], player)
		if total >= 4 {
			return true
		}
	}
	return false
}

func isBoardFull(board [][]int) bool {
	for _, row := range board {
		for _, cell := range row {
			if cell == 0 {
				return false
			}
		}
	}
	return true
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func readJSON[T any](path string) (T, error) {
	var zero T
	f, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	var v T
	if err := dec.Decode(&v); err != nil {
		return zero, err
	}
	return v, nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func itoa(i int) string { return fmtSprintf("%d", i) }

func fmtSscanf(s string, v *int) (int, error)     { return fmtSscanfImpl(s, v) }
func fmtSscanfImpl(s string, v *int) (int, error) { return fmtSscanfReal(s, v) }

var (
	fmtSscanfReal = func(s string, v *int) (int, error) { return fmtSscanfStd(s, v) }
	fmtSprintf    = func(format string, a ...any) string { return fmtSprintfStd(format, a...) }
)

func fmtSscanfStd(s string, v *int) (int, error)   { return fmt.Sscanf(s, "%d", v) }
func fmtSprintfStd(format string, a ...any) string { return fmt.Sprintf(format, a...) }

func (a *App) mustParse(name string) *template.Template {
	path := filepath.Join(a.templateDir, name)
	tpl, err := template.New(name).Funcs(template.FuncMap{
		"seq": func(i, j int) []int {
			s := make([]int, j-i+1)
			for x := range s {
				s[x] = i + x
			}
			return s
		},
		"cellClass": func(cell int) string {
			if cell == 1 {
				return "p1"
			}
			if cell == 2 {
				return "p2"
			}
			return "empty"
		},
		"isLast": func(m *Move, r, c int) bool {
			if m == nil {
				return false
			}
			return m.Row == r && m.Col == c
		},
	}).ParseFiles(path)
	if err != nil {
		panic(err)
	}
	return tpl
}

func (a *App) render(w http.ResponseWriter, name string, _ any) {
	tpl := a.mustParse(name)
	if err := tpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) renderWithData(w http.ResponseWriter, name string, data any) {
	tpl := a.mustParse(name)
	if err := tpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
