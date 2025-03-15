package interactive

import (
	goroom "github.com/jdginn/go-recording-studio/room"
)

func interact(arrivals []goroom.Arrival) error {
	tea.NewProgram(interactModel{arrivals: arrivals}).Start()
}
