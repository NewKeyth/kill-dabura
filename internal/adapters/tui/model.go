package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"dabura/internal/core/domain"
	"dabura/internal/core/services"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

type State int

const (
	Searching State = iota
	Results
	Menu
	ServerSelection
	Resolving
	InputFileName
	ExtractingInfo
	Downloading
	Success
)

type Model struct {
	State        State
	TextInput    textinput.Model
	Table        table.Model
	Viewport     viewport.Model
	Movies       []domain.Movie
	Selected     int
	MenuCursor   int
	Loading      bool
	Languages    []string
	LangIndex    int
	Width         int
	Height        int
	Err           error
	Action        string
	// Multi-provider state
	Providers     []string
	ProviderIndex int
	StreamOptions []domain.StreamOption
	ServerCursor  int

	service      *services.MovieService
	cancelSearch context.CancelFunc

	// File Input & Download Progress
	FileNameInput textinput.Model
	DownloadMsg   chan tea.Msg
	Percent       float64
	Speed         string
	ETA           string
	TargetFile    string
	Merging       bool
	Spinner       spinner.Model
}

type progressMsg struct {
	Percent float64
	Speed   string
	ETA     string
	Err     error
}

type successMsg struct {
	Details string
}

func InitialModel(svc *services.MovieService) Model {
	ti := textinput.New()
	ti.Placeholder = "Nombre de la película..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	langs := svc.GetLanguages()
	var provs []string
	if len(langs) > 0 {
		provs = svc.GetProvidersForLanguage(langs[0])
	}

	fns := textinput.New()
	fns.Placeholder = "Ejemplo: matrix_1080p.mp4"
	fns.CharLimit = 100
	fns.Width = 40

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF06B7"))

	return Model{
		State:         Searching,
		TextInput:     ti,
		Languages:     langs,
		LangIndex:     0,
		Providers:     provs,
		ProviderIndex: 0,
		MenuCursor:    0,
		service:       svc,
		FileNameInput: fns,
		DownloadMsg:   make(chan tea.Msg),
		Spinner:       s,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.Spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Viewport = viewport.New(m.Width-8, m.Height-4)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.cancelSearch != nil {
				m.cancelSearch()
			}
			return m, tea.Quit
		case "esc":
			if m.State == Resolving && m.cancelSearch != nil {
				m.cancelSearch()
				m.State = Menu
				return m, nil
			}
			if m.State == ServerSelection {
				m.State = Menu
				return m, nil
			}
			if m.Loading && m.cancelSearch != nil {
				m.cancelSearch()
				m.Loading = false
				m.State = Searching
				m.TextInput.Focus()
				return m, nil
			}
			if m.State == Results {
				m.State = Searching
				m.TextInput.Focus()
			} else if m.State == Menu {
				m.State = Results
			} else {
				return m, tea.Quit
			}
		case "tab":
			if m.State == Searching {
				m.LangIndex = (m.LangIndex + 1) % len(m.Languages)
				// Actualizar la lista de proveedores para este nuevo idioma
				if len(m.Languages) > 0 {
					m.Providers = m.service.GetProvidersForLanguage(m.Languages[m.LangIndex])
					m.ProviderIndex = 0
				}
			}
		case "left":
			if (m.State == Searching || m.State == Results) && len(m.Providers) > 0 {
				m.ProviderIndex--
				if m.ProviderIndex < 0 {
					m.ProviderIndex = len(m.Providers) - 1
				}
				if m.TextInput.Value() != "" {
					m.Loading = true
					return m, m.searchCmd(m.TextInput.Value())
				}
			}
		case "right":
			if (m.State == Searching || m.State == Results) && len(m.Providers) > 0 {
				m.ProviderIndex = (m.ProviderIndex + 1) % len(m.Providers)
				if m.TextInput.Value() != "" {
					m.Loading = true
					return m, m.searchCmd(m.TextInput.Value())
				}
			}
		case "up":
			if m.State == Results {
				m.Table, cmd = m.Table.Update(msg)
				return m, cmd
			}
			if m.State == Menu && m.MenuCursor > 0 {
				m.MenuCursor--
			}
			if m.State == ServerSelection && m.ServerCursor > 0 {
				m.ServerCursor--
			}
		case "down":
			if m.State == Results {
				m.Table, cmd = m.Table.Update(msg)
				return m, cmd
			}
			if m.State == Menu && m.MenuCursor < 2 {
				m.MenuCursor++
			}
			if m.State == ServerSelection && m.ServerCursor < len(m.StreamOptions)-1 {
				m.ServerCursor++
			}
		case "enter":
			if m.State == Searching && m.TextInput.Value() != "" {
				m.Loading = true
				m.TextInput.Blur()
				return m, m.searchCmd(m.TextInput.Value())
			}
			if m.State == Results && len(m.Movies) > 0 {
				m.Selected = m.Table.Cursor()
				m.State = Menu
				m.MenuCursor = 0
			} else if m.State == Menu {
				movie := m.Movies[m.Selected]
				m.State = Resolving
				m.Err = nil
				if m.MenuCursor == 0 {
					m.Action = "play"
				} else if m.MenuCursor == 1 {
					m.Action = "browser"
				} else {
					m.Action = "download"
				}
				// We don't resolve directly to process anymore, we get the StreamOptions
				return m, m.resolveCmd(movie)
			} else if m.State == ServerSelection {
				if m.Action == "download" {
					m.State = InputFileName
					m.FileNameInput.Focus()
					return m, nil
				}
				return m, m.executeSelectedServer()
			} else if m.State == InputFileName {
				if m.FileNameInput.Value() != "" {
					m.TargetFile = m.FileNameInput.Value()
					if !strings.HasSuffix(m.TargetFile, ".mp4") {
						m.TargetFile += ".mp4"
					}
					m.State = ExtractingInfo
					return m, m.startDownloadCmd()
				}
			} else if m.State == Success {
				// Volver al inicio despues del exito
				m.State = Searching
				m.TextInput.Focus()
				return m, nil
			}
		}
	case spinner.TickMsg:
		var cmdSpinner tea.Cmd
		m.Spinner, cmdSpinner = m.Spinner.Update(msg)
		return m, cmdSpinner

	case progressMsg:
		if msg.Err != nil {
			m.Err = msg.Err
			m.State = Menu
			return m, nil
		}
		m.State = Downloading
		m.Percent = msg.Percent
		m.Speed = msg.Speed
		m.ETA = msg.ETA
		if msg.Percent >= 100.0 {
			m.Merging = true
		}
		return m, m.waitForProgressCmd()

	case successMsg:
		m.State = Success
		return m, nil

	case searchResultMsg:
		m.Loading = false
		m.cancelSearch = nil
		if msg.err != nil {
			m.Err = msg.err
			m.State = Searching
			m.TextInput.Focus()
		} else {
			m.Movies = msg.movies
			m.State = Results
			m.Table = m.initTable()
		}

	case resolveResultMsg:
		m.cancelSearch = nil
		if msg.err != nil {
			m.Err = msg.err
			m.State = Menu
			return m, nil
		}
		
		m.StreamOptions = msg.options
		
		if len(m.StreamOptions) > 1 {
			m.State = ServerSelection
			m.ServerCursor = 0
			return m, nil
		}

		// Exactamente una opción, ejecuta directo
		m.ServerCursor = 0
		return m, m.executeSelectedServer()

	case processFinishedMsg:
		m.State = Menu
		if msg.err != nil {
			m.Err = msg.err
		}
		return m, nil
	}

	if m.State == Searching {
		m.TextInput, cmd = m.TextInput.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.State == InputFileName {
		m.FileNameInput, cmd = m.FileNameInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.Viewport, cmd = m.Viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) initTable() table.Model {
	tableWidth := m.Width - 14
	if tableWidth < 60 { tableWidth = 60 }
	columns := []table.Column{
		{Title: "TITULO", Width: int(float64(tableWidth) * 0.5)},
		{Title: "AÑO", Width: 8},
		{Title: "RATING", Width: 8},
		{Title: "CALIDAD", Width: 10},
		{Title: "IDIOMA", Width: 12},
	}
	var rows []table.Row
	for _, movie := range m.Movies {
		rows = append(rows, table.Row{movie.Title, movie.Year, movie.Rating, movie.Quality, movie.Language})
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(m.Height-14),
	)
	return t
}

type searchResultMsg struct {
	movies []domain.Movie
	err    error
}
type resolveResultMsg struct {
	options []domain.StreamOption
	err     error
}
type processFinishedMsg struct {
	err error
}

func (m *Model) searchCmd(query string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelSearch = cancel

	providerName := ""
	if len(m.Providers) > 0 {
		providerName = m.Providers[m.ProviderIndex]
	}

	return func() tea.Msg {
		movies, err := m.service.SearchProvider(ctx, providerName, query)
		return searchResultMsg{movies: movies, err: err}
	}
}

func (m *Model) executeSelectedServer() tea.Cmd {
	selected := m.StreamOptions[m.ServerCursor].URL
	ctx := context.Background()
	var execCmd *exec.Cmd

	if m.Action == "play" {
		execCmd = m.service.GetPlayCmd(ctx, selected, false)
	} else if m.Action == "browser" {
		execCmd = m.service.GetPlayCmd(ctx, selected, true)
	}

	return tea.ExecProcess(execCmd, func(err error) tea.Msg {
		return processFinishedMsg{err}
	})
}

func (m *Model) waitForProgressCmd() tea.Cmd {
	return func() tea.Msg {
		return <-m.DownloadMsg
	}
}

func (m *Model) startDownloadCmd() tea.Cmd {
	selected := m.StreamOptions[m.ServerCursor].URL
	home, _ := os.UserHomeDir()
	fullPath := home + "/Downloads/" + m.TargetFile
	
	execCmd := m.service.GetDownloadCmd(context.Background(), selected, "1080", fullPath)

	return func() tea.Msg {
		stdout, err := execCmd.StdoutPipe()
		if err != nil {
			return progressMsg{Err: fmt.Errorf("error al interceptar salida: %v", err)}
		}

		if err := execCmd.Start(); err != nil {
			return progressMsg{Err: fmt.Errorf("error al iniciar yt-dlp: %v", err)}
		}

		go func() {
			scanner := bufio.NewScanner(stdout)
			rePercent := regexp.MustCompile(`\[download\]\s+([0-9.]+)%`)
			reSpeed := regexp.MustCompile(`at\s+([0-9.]+[a-zA-Z]+/s)`)
			reETA := regexp.MustCompile(`ETA\s+([0-9:]+)`)

			for scanner.Scan() {
				line := scanner.Text()
				
				pctMatch := rePercent.FindStringSubmatch(line)
				spdMatch := reSpeed.FindStringSubmatch(line)
				etaMatch := reETA.FindStringSubmatch(line)

				msg := progressMsg{}
				updated := false

				if len(pctMatch) > 1 {
					if pct, err := strconv.ParseFloat(pctMatch[1], 64); err == nil {
						msg.Percent = pct
						updated = true
					}
				}
				if len(spdMatch) > 1 {
					msg.Speed = spdMatch[1]
					updated = true
				}
				if len(etaMatch) > 1 {
					msg.ETA = etaMatch[1]
					updated = true
				}

				if updated {
					m.DownloadMsg <- msg
				}
			}

			err := execCmd.Wait()
			if err != nil {
				m.DownloadMsg <- progressMsg{Err: fmt.Errorf("error en descarga: %v", err)}
			} else {
				m.DownloadMsg <- successMsg{Details: fullPath}
			}
		}()

		// Initiate the first read loop
		return <-m.DownloadMsg
	}
}

func (m *Model) resolveCmd(movie domain.Movie) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelSearch = cancel

	return func() tea.Msg {
		options, err := m.service.ExtractStreamURLs(ctx, movie)
		return resolveResultMsg{options: options, err: err}
	}
}
