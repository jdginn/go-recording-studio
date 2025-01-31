package room

import (
	// "fmt"
	// "sort"

	"fmt"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"

	"github.com/fogleman/gg"
	"github.com/fogleman/pt/pt"
)

type View struct {
	C          *gg.Context
	TranslateX float64
	TranslateY float64
	Scale      float64
	Plane      Plane
}

func (o View) project(v pt.Vector) Point2D {
	return To2D(o.Plane.Project(v))
}

func (o View) scaleAndTranslate(p Point2D) Point2D {
	return p.Scale(o.Scale).Translate(o.TranslateX, o.TranslateY)
}

func (o View) Save(path string) error {
	err := o.C.SavePNG(path)
	if err != nil {
		return err
	}
	o.C.Clear()
	return nil
}

type Scene struct {
	Sources           []Speaker
	ListeningPosition pt.Vector
	Room              *Room
}

func (scene Scene) PlotArrivals3D(arrivals []Arrival, view View) {
	listenPos := view.scaleAndTranslate(view.project(scene.ListeningPosition))
	view.C.DrawCircle(listenPos.X, listenPos.Y, 2)
	view.C.Fill()
	view.C.DrawCircle(listenPos.X, listenPos.Y, 50)
	for _, source := range scene.Sources {
		sourcePos := view.scaleAndTranslate(view.project(source.Position))
		view.C.DrawCircle(sourcePos.X, sourcePos.Y, 2)
	}

	for _, lines := range view.Plane.MeshToPath(scene.Room.M) {
		for i := 0; i < len(lines)-1; i++ {
			p1 := view.scaleAndTranslate(lines[i])
			p2 := view.scaleAndTranslate(lines[i+1])
			view.C.DrawLine(p1.X, p1.Y, p2.X, p2.Y)
			view.C.Stroke()
		}
	}

	for _, arrival := range arrivals {
		if arrival.Distance == INF {
			continue
		}
		positions := arrival.AllReflections

		p1 := view.scaleAndTranslate(view.project(arrival.Shot.ray.Origin))
		for i := 0; i < len(positions); i++ {
			p2 := view.scaleAndTranslate(view.project(positions[i]))
			view.C.DrawLine(p1.X, p1.Y, p2.X, p2.Y)
			p1 = p2
		}
		p2 := view.scaleAndTranslate(view.project(arrival.NearestApproachPosition))
		view.C.DrawLine(p1.X, p1.Y, p2.X, p2.Y)
		view.C.Stroke()
	}
}

type valuer struct {
	data   map[int]float64
	length int
}

func (v *valuer) AddArrival(delay float64, gain float64) {
	v.data[int(delay*1000)] = gain + 20
}

func (v valuer) Len() int {
	return v.length
}

func (v valuer) Value(i int) float64 {
	if gain, ok := v.data[i]; ok {
		return gain
	}
	return 0
}

func (scene Scene) PlotITD(arrivals []Arrival, window int) error {
	p := plot.New()
	p.Title.Text = "ITD"
	p.X.Label.Text = "Time (ms)"
	p.Y.Label.Text = "Reflection gain (dB)"

	const MS = 1.0 / 1000.0
	directDist := scene.ListeningPosition.Sub(scene.Sources[0].Position).Length()

	v := valuer{
		data:   map[int]float64{},
		length: window * 1000,
	}

	for _, arrival := range arrivals {
		delay := arrival.Distance - directDist/SPEED_OF_SOUND*MS
		v.AddArrival(delay, arrival.Gain)
		fmt.Printf("%f ms %f dB\n", delay, arrival.Gain)
	}

	itd, err := plotter.NewBarChart(v, vg.Points(3))
	if err != nil {
		return err
	}
	p.Add(itd)
	p.Save(1000, 1000, "itd.png")

	return nil
}
