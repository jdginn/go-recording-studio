package main

import (
	"image/color"
	"os"
	"path"
	"sort"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/fogleman/gg"

	goroom "github.com/jdginn/go-recording-studio/room"
)

func mustCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return cwd
}

const MS float64 = 1.0 / 1000.0

const SCALE float64 = 100

var (
	BRICK         = goroom.Material{Alpha: 0.04}
	WOOD          = goroom.Material{Alpha: 0.1}
	GYPSUM        = goroom.Material{Alpha: 0.05}
	DIFFUSER      = goroom.Material{Alpha: 0.99}
	ROCKWOOL_12CM = goroom.Material{Alpha: 0.9}
	ROCKWOOL_24CM = goroom.Material{Alpha: 0.996}
	ROCKWOOL_30CM = goroom.Material{Alpha: 0.999}
	GLASS         = goroom.Material{Alpha: 0.0}
)

type GUIMODE int

const (
	GUI_MODE_INITIAL GUIMODE = iota
	GUI_MODE_FILE_LOADED
	GUI_MODE_SIMULATED
)

type canvasBuilder struct {
	hasFilename  bool
	hasSimulated bool
}

type GuiContext struct {
	// TODO: use this
	sync.Mutex
	w        fyne.Window
	obj      fyne.CanvasObject
	guimode  GUIMODE
	Filename string
	Scene    goroom.Scene
	Arrivals []goroom.Arrival
}

func NewGuiContext(w fyne.Window) *GuiContext {
	c := &GuiContext{
		w: w,
	}
	c.update(GUI_MODE_INITIAL)
	return c
}

func (c *GuiContext) load(filename string) error {
	c.Filename = filename
	room, err := goroom.NewFrom3MF(filename, map[string]goroom.Material{
		"default":            BRICK,
		"Floor":              WOOD,
		"Front A":            GYPSUM,
		"Front B":            GYPSUM,
		"Back Diffuser":      DIFFUSER,
		"Ceiling Diffuser":   ROCKWOOL_24CM,
		"Street A":           ROCKWOOL_24CM,
		"Street B":           ROCKWOOL_24CM,
		"Street C":           ROCKWOOL_24CM,
		"Street D":           ROCKWOOL_24CM,
		"Street E":           ROCKWOOL_24CM,
		"Hall A":             ROCKWOOL_24CM,
		"Hall B":             ROCKWOOL_24CM,
		"Hall E":             ROCKWOOL_24CM,
		"Entry Back":         ROCKWOOL_24CM,
		"Entry Front":        ROCKWOOL_24CM,
		"Cutout Top":         ROCKWOOL_24CM,
		"Window A":           GLASS,
		"Window B":           GLASS,
		"left speaker wall":  GYPSUM,
		"right speaker wall": GYPSUM,
	})
	if err != nil {
		return err
	}
	c.Scene = goroom.Scene{
		Room: &room,
	}
	c.update(GUI_MODE_FILE_LOADED)
	return nil
}

func (c *GuiContext) simulate() error {
	lt := goroom.ListeningTriangle{
		ReferencePosition: goroom.V(0, 2.0, 0.5),
		ReferenceNormal:   goroom.V(1, 0, 0),
		DistFromFront:     0.526,
		DistFromCenter:    1.3,
		SourceHeight:      1.86,
		ListenHeight:      1.4,
	}

	mum8Spec := goroom.LoudSpeakerSpec{
		Xdim:        0.38,
		Ydim:        0.256,
		Zdim:        0.52,
		Yoff:        0.096,
		Zoff:        0.412,
		Directivity: goroom.NewDirectivity(map[float64]float64{0: 0, 30: 0, 60: -12, 70: -100}, map[float64]float64{0: 0, 30: -9, 60: -15, 70: -19, 80: -30}),
	}

	sources := []goroom.Speaker{
		goroom.NewSpeaker(mum8Spec, lt.LeftSourcePosition(), lt.LeftSourceNormal()),
		goroom.NewSpeaker(mum8Spec, lt.RightSourcePosition(), lt.RightSourceNormal()),
	}

	arrivals := []goroom.Arrival{}

	for _, source := range sources {
		ok, err := source.IsInsideRoom(c.Scene.Room.M, lt.ListenPosition())
		if err != nil {
			panic(err)
		}
		if !ok {
			panic("Speaker is not inside room!")
		}
	}

	for _, source := range sources {
		for _, shot := range source.Sample(100, 180, 180) {
			arrival, err := c.Scene.Room.TraceShot(shot, lt.ListenPosition(), goroom.TraceParams{
				Order:         10,
				GainThreshold: -20,
				TimeThreshold: 100 * MS,
				RFZRadius:     0.5,
			})
			if err != nil {
				panic(err)
			}
			if arrival.Distance != goroom.INF {
				arrivals = append(arrivals, arrival)
			}
		}
	}

	c.Scene.ListeningTriangle = lt
	c.Scene.ListeningPosition = lt.ListenPosition()
	c.Scene.Sources = sources
	c.Arrivals = arrivals
	c.update(GUI_MODE_SIMULATED)
	return nil
}

func (c *GuiContext) drawReflections(arrivals []goroom.Arrival) error {
	sort.Slice(arrivals, func(i int, j int) bool {
		return arrivals[i].Distance < arrivals[j].Distance
	})

	p1 := goroom.MakePlane(goroom.V(0.25, 0.5, 0), goroom.V(0, 1, 0))
	p2 := goroom.MakePlane(goroom.V(0.25, 0.5, 0.75), goroom.V(0, 0, 1))

	view := goroom.View{
		C:          gg.NewContext(1000, 1000),
		TranslateX: 400,
		TranslateY: 400,
		Scale:      100,
		Plane:      p1,
	}

	// interact.Interact(c.Scene, view, arrivals, c.Scene.ListeningTriangle.ListenDistance())

	c.Scene.PlotArrivals3D(arrivals, view)
	view.Save(path.Join(mustCwd(), "out1.png"))
	view.Plane = p2
	c.Scene.PlotArrivals3D(arrivals, view)
	view.Save(path.Join(mustCwd(), "out2.png"))
	return nil
}

func (c *GuiContext) update(mode GUIMODE) {
	status := widget.NewLabel("goroom recording studio design")
	drawingFromTop := canvas.NewImageFromFile(path.Join(mustCwd(), "out1.png"))
	drawingFromTop.FillMode = canvas.ImageFillContain
	drawingFromTop.Resize(fyne.Size{Width: 400, Height: 300})
	drawingFromTop.SetMinSize(fyne.Size{Width: 400, Height: 300})

	itd := canvas.NewImageFromFile(path.Join(mustCwd(), "itd.png"))
	itd.FillMode = canvas.ImageFillContain
	itd.Resize(fyne.Size{Width: 400, Height: 300})
	itd.SetMinSize(fyne.Size{Width: 400, Height: 300})

	switch mode {
	case GUI_MODE_INITIAL:
		c.obj = container.NewVBox(
			status,
			widget.NewButton("Import room model", func() {
				fd := dialog.NewFileOpen(func(f fyne.URIReadCloser, err error) {
					if err != nil {
						dialog.ShowError(err, c.w)
						return
					}
					if f == nil {
						return
					}
					c.load(f.URI().Path())
					status.SetText(c.Filename)
				}, c.w)

				cwd, err := os.Getwd()
				if err != nil {
					dialog.ShowError(err, c.w)
					return
				}
				luri, err := storage.ListerForURI(storage.NewFileURI(cwd))
				if err != nil {
					dialog.ShowError(err, c.w)
					return
				}
				fd.SetLocation(luri)
				fd.Show()
			}),
		)
	case GUI_MODE_FILE_LOADED:
		c.obj = container.NewVBox(
			status,
			widget.NewButton("Simulate", func() {
				if err := c.simulate(); err != nil {
					dialog.ShowError(err, c.w)
				}
				c.drawReflections(c.Arrivals)
				c.w.Content().Refresh()
			}),
		)
	case GUI_MODE_SIMULATED:
		c.obj = container.NewVBox(
			widget.NewList(func() int { return len(c.Arrivals) }, func() fyne.CanvasObject { return canvas.NewCircle(color.Black) }, func(i widget.ListItemID, o fyne.CanvasObject) {}),
			widget.NewLabel("Top view:"),
			drawingFromTop,
			widget.NewLabel("ITD graph:"),
			itd,
		)
	}
	c.obj.Refresh()
	c.w.SetContent(c.obj)
}

func main() {
	a := app.New()
	w := a.NewWindow("goroom recording studio design software")
	w.Resize(fyne.Size{
		Width:  600,
		Height: 600,
	})

	_ = NewGuiContext(w)

	w.ShowAndRun()
}
