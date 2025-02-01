package interact

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	goroom "github.com/jdginn/go-recording-studio/room"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type item struct {
	arrival goroom.Arrival
	delayMs float64
}

func (i item) Title() string {
	return fmt.Sprintf("%f ms %f dB", i.delayMs, i.arrival.Gain)
}

func (i item) Description() string {
	return fmt.Sprintf("%d reflections", len(i.arrival.AllReflections))
}

func (i item) FilterValue() string {
	return i.Title()
}

type model struct {
	list  list.Model
	scene goroom.Scene
	view  goroom.View
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	i := []goroom.Arrival{m.list.SelectedItem().(item).arrival}
	m.scene.PlotArrivals3D(i, m.view)
	m.view.Save("out1.png")
	m.view.Plane = goroom.MakePlane(goroom.V(0.25, 0.5, 0.75), goroom.V(0, 0, 1))
	m.scene.PlotArrivals3D(i, m.view)
	m.view.Save("out2.png")
	return m, cmd
}

func (m model) View() string {
	return docStyle.Render(m.list.View())
}

func Interact(scene goroom.Scene, view goroom.View, arrivals []goroom.Arrival, directDist float64) error {
	const MS = 1.0 / 1000.0
	items := make([]list.Item, len(arrivals))
	for i, arrival := range arrivals {
		delay := arrival.Distance - directDist/goroom.SPEED_OF_SOUND*MS
		items[i] = item{arrival: arrival, delayMs: delay}
	}

	m := model{list: list.New(items, list.NewDefaultDelegate(), 0, 0), scene: scene, view: view}
	m.list.Title = "My Fave Things"

	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
	return nil
}
