package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

var (
	pink   = lipgloss.Color("#FF06B7")
	white  = lipgloss.Color("#FFFFFF")
	gray   = lipgloss.Color("#444444")

	containerStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(white).
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().Foreground(pink).Bold(true)
	textStyle  = lipgloss.NewStyle().Foreground(white)
	infoStyle  = lipgloss.NewStyle().Foreground(gray).Italic(true)
	
	searchBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(white).
			Padding(0, 1)

	selectedMenuStyle = lipgloss.NewStyle().
				Foreground(pink).
				Background(white).
				PaddingLeft(1).
				Bold(true)
)

const asciiTitle = `
██████╗  █████╗ ██████╗ ██╗   ██╗██████╗  █████╗ 
██╔══██╗██╔══██╗██╔══██╗██║   ██║██╔══██╗██╔══██╗
██║  ██║███████║██████╔╝██║   ██║██████╔╝███████║
██║  ██║██╔══██║██╔══██╗██║   ██║██╔══██╗██╔══██║
██████╔╝██║  ██║██████╔╝╚██████╔╝██║  ██║██║  ██║
╚═════╝ ╚═╝  ╚═╝╚═════╝  ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝
`

func (m Model) View() string {
	var inner string
	
	boxWidth := m.Width - 2
	boxHeight := m.Height - 2
	if boxWidth < 30 {
		boxWidth = 30
	}
	if boxHeight < 15 {
		boxHeight = 15
	}

	effWidth := boxWidth - 6
	effHeight := boxHeight - 4

	if m.Loading {
		inner = textStyle.Render("BUSCANDO...")
	} else if m.State == Resolving {
		inner = lipgloss.JoinVertical(lipgloss.Center,
			titleStyle.Render("BYPASSEANDO CLOUDFLARE Y EXTRAYENDO STREAM..."),
			"",
			textStyle.Render("Por favor espera... Esto puede tomar hasta 20 segundos."),
			infoStyle.Render("(Chromedp está buscando el .m3u8 en la red de forma invisible)"),
			"",
			infoStyle.Render("(Presiona ESC para abortar si dura demasiado)"),
		)
	} else {
		switch m.State {
		case Searching:
			var langs []string
			for i, l := range m.Languages {
				if i == m.LangIndex {
					langs = append(langs, lipgloss.NewStyle().Foreground(pink).Bold(true).Render("["+l+"]"))
				} else {
					langs = append(langs, lipgloss.NewStyle().Foreground(gray).Render(" "+l+" "))
				}
			}
			langLine := strings.Join(langs, " ")

			var provs []string
			for i, p := range m.Providers {
				if i == m.ProviderIndex {
					provs = append(provs, lipgloss.NewStyle().Foreground(pink).Bold(true).Render("["+p+"]"))
				} else {
					provs = append(provs, lipgloss.NewStyle().Foreground(gray).Render(" "+p+" "))
				}
			}
			provLine := strings.Join(provs, " | ")

			errText := ""
			if m.Err != nil {
				errText = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(fmt.Sprintf("%v", m.Err))
			}

			inner = lipgloss.JoinVertical(lipgloss.Center,
				titleStyle.Render(asciiTitle),
				"", 
				textStyle.Render("PROVEEDORES: (Flechas L/R)"),
				provLine,
				"",
				searchBoxStyle.Width(effWidth/2).Render(m.TextInput.View()),
				"",
				textStyle.Render("IDIOMAS SOPORTADOS: (Tab)"),
				langLine,
				"", "",
				infoStyle.Render("(Enter = Buscar • Esc = Salir)"),
				errText,
			)

		case Results:
			st := table.DefaultStyles()
			st.Header = st.Header.
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(white).
				BorderBottom(true).
				Bold(true).
				Foreground(white)
			st.Selected = st.Selected.
				Foreground(pink).
				Background(white).
				Bold(true)
			m.Table.SetStyles(st)

			var provs []string
			for i, p := range m.Providers {
				if i == m.ProviderIndex {
					provs = append(provs, lipgloss.NewStyle().Foreground(pink).Bold(true).Render("["+p+"]"))
				} else {
					provs = append(provs, lipgloss.NewStyle().Foreground(gray).Render(" "+p+" "))
				}
			}
			provLine := strings.Join(provs, " | ")

			inner = lipgloss.JoinVertical(lipgloss.Center,
				textStyle.Render(fmt.Sprintf("RESULTADOS PARA: %s", strings.ToUpper(m.TextInput.Value()))),
				provLine,
				"",
				m.Table.View(),
				"",
				infoStyle.Render("(↑/↓ Navegar • L/R Cambiar Proveedor • Enter Seleccionar • Esc Volver)"),
			)

		case Menu:
			selectedMovie := m.Movies[m.Selected]
			
			opt1 := textStyle.Render(" 1. VER PELÍCULA (REPRODUCTOR MPV/VLC AUTO) ")
			opt2 := textStyle.Render(" 2. VER EN NAVEGADOR CLÁSICO ")
			opt3 := textStyle.Render(" 3. DESCARGAR PELÍCULA (YT-DLP) ")
			
			if m.MenuCursor == 0 {
				opt1 = selectedMenuStyle.Render(" 1. VER PELÍCULA (REPRODUCTOR MPV/VLC AUTO) ")
			} else if m.MenuCursor == 1 {
				opt2 = selectedMenuStyle.Render(" 2. VER EN NAVEGADOR CLÁSICO ")
			} else {
				opt3 = selectedMenuStyle.Render(" 3. DESCARGAR PELÍCULA (YT-DLP) ")
			}
			
			errText := ""
			if m.Err != nil {
				errText = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(fmt.Sprintf("ERROR: %v", m.Err))
			}

			inner = lipgloss.JoinVertical(lipgloss.Center,
				textStyle.Render("PELÍCULA: "+strings.ToUpper(selectedMovie.Title)),
				"",
				textStyle.Render("OPCIONES:"),
				"",
				opt1,
				opt2,
				opt3,
				"",
				errText,
				"",
				infoStyle.Render("(↑/↓ Navegar • Enter Ejecutar • Esc Volver)"),
			)

		case ServerSelection:
			selectedMovie := m.Movies[m.Selected]

			var srvOpts []string
			for i, opt := range m.StreamOptions {
				cursor := "  "
				renderOpt := textStyle.Render(fmt.Sprintf("%s (%s)", opt.Server, opt.Quality))
				if i == m.ServerCursor {
					cursor = "> "
					renderOpt = selectedMenuStyle.Render(fmt.Sprintf("%s (%s)", strings.ToUpper(opt.Server), opt.Quality))
				}
				srvOpts = append(srvOpts, cursor+renderOpt)
			}

			inner = lipgloss.JoinVertical(lipgloss.Center,
				textStyle.Render("PELÍCULA: "+strings.ToUpper(selectedMovie.Title)),
				"",
				titleStyle.Render("MÚLTIPLES SERVIDORES DETECTADOS"),
				textStyle.Render("Elige desde dónde descargar/reproducir:"),
				"",
				strings.Join(srvOpts, "\n"),
				"",
				infoStyle.Render("(↑/↓ Seleccionar Servidor • Enter Confirmar • Esc Volver)"),
			)

		case InputFileName:
			inner = lipgloss.JoinVertical(lipgloss.Center,
				titleStyle.Render("NOMBRE DE ARCHIVO"),
				"",
				textStyle.Render("Escribe el nombre para tu descarga (se guardará en Descargas):"),
				"",
				searchBoxStyle.Width(effWidth/2).Render(m.FileNameInput.View()),
				"",
				infoStyle.Render("(Enter = Confirmar • Esc = Volver al menú)"),
			)

		case ExtractingInfo:
			inner = lipgloss.JoinVertical(lipgloss.Center,
				lipgloss.NewStyle().Foreground(pink).Bold(true).Render("Recopilando información..."),
				"",
				infoStyle.Render("(Conectando con el servidor de video)"),
			)

		case Downloading:
			var renderBody string
			if m.Merging {
				renderBody = lipgloss.JoinVertical(lipgloss.Center,
					titleStyle.Render("AJUSTES FINALES"),
					"",
					textStyle.Render(fmt.Sprintf("Destino: %s", m.TargetFile)),
					"",
					m.Spinner.View()+" "+lipgloss.NewStyle().Foreground(white).Bold(true).Render("Uniendo audio y video en contenedor MP4..."),
					lipgloss.NewStyle().Foreground(pink).Bold(true).Render("Por favor NO CIERRE la ventana. Este proceso toma unos minutos."),
				)
			} else {
				// Simple progress bar
				barWidth := 40
				filled := int((m.Percent / 100.0) * float64(barWidth))
				if filled < 0 {
					filled = 0
				}
				if filled > barWidth {
					filled = barWidth
				}
				empty := barWidth - filled

				barStr := strings.Repeat("█", filled) + strings.Repeat("░", empty)
				progStr := fmt.Sprintf("[%s] %.1f%%", barStr, m.Percent)

				renderBody = lipgloss.JoinVertical(lipgloss.Center,
					titleStyle.Render("DESCARGANDO PELÍCULA"),
					"",
					textStyle.Render(fmt.Sprintf("Destino: %s", m.TargetFile)),
					"",
					lipgloss.NewStyle().Foreground(pink).Render(progStr),
					"",
					textStyle.Render(fmt.Sprintf("Velocidad: %s  |  ETA: %s", m.Speed, m.ETA)),
				)
			}
			inner = renderBody

		case Success:
			inner = lipgloss.JoinVertical(lipgloss.Center,
				lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true).Render("¡DESCARGADO CON ÉXITO!"),
				"",
				textStyle.Render(fmt.Sprintf("El archivo %s está listo en tu carpeta de Descargas.", m.TargetFile)),
				"",
				infoStyle.Render("(Presione Enter o Esc para regresar al inicio)"),
			)
		}
	}

	m.Viewport.Width = effWidth
	m.Viewport.Height = effHeight - 1 // Leave 1 line for the bottom branding
	content := lipgloss.Place(effWidth, m.Viewport.Height, lipgloss.Center, lipgloss.Center, inner)
	m.Viewport.SetContent(content)
	
	// Bottom Bar 
	brandLine := lipgloss.NewStyle().Foreground(pink).Render("▶ ") + 
		lipgloss.NewStyle().Foreground(gray).Render("KEYTHDZNG daburav1.0.0")
	bottomBar := lipgloss.PlaceHorizontal(effWidth, lipgloss.Right, brandLine)

	mainContainer := lipgloss.JoinVertical(lipgloss.Left,
		m.Viewport.View(),
		bottomBar,
	)

	// Since mainContainer is exactly effWidth x effHeight, the border will hug it perfectly
	ui := containerStyle.Render(mainContainer)
	
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, ui)
}
