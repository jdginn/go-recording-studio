package room

import (
	// "sort"

	"fmt"
	"image"
	"image/png"
	"os"
	"path"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"

	"github.com/fogleman/gg"
	"github.com/fogleman/pt/pt"
)

type Size struct {
	X int
	Y int
}

type View struct {
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

type Scene struct {
	Sources           []Speaker
	ListeningPosition pt.Vector
	ListeningTriangle ListeningTriangle
	Room              *Room
}

func (scene Scene) PlotArrivals3D(X, Y int, arrivals []Arrival, view View) (image.Image, error) {
	c := gg.NewContext(X, Y)

	listenPos := view.scaleAndTranslate(view.project(scene.ListeningPosition))
	c.DrawCircle(listenPos.X, listenPos.Y, 2)
	c.Fill()
	c.DrawCircle(listenPos.X, listenPos.Y, 50)
	for _, source := range scene.Sources {
		sourcePos := view.scaleAndTranslate(view.project(source.Position))
		c.DrawCircle(sourcePos.X, sourcePos.Y, 2)
	}

	for _, lines := range view.Plane.MeshToPath(scene.Room.M) {
		for i := 0; i < len(lines)-1; i++ {
			p1 := view.scaleAndTranslate(lines[i])
			p2 := view.scaleAndTranslate(lines[i+1])
			c.SetLineWidth(10)
			c.DrawLine(p1.X, p1.Y, p2.X, p2.Y)
			c.Stroke()
		}
	}

	for _, arrival := range arrivals {
		if arrival.Distance == INF {
			continue
		}
		positions := arrival.AllReflections

		p1 := view.scaleAndTranslate(view.project(arrival.Shot.ray.Origin))
		for i := 0; i < len(positions); i++ {
			c.SetLineWidth(arrival.Gain)
			p2 := view.scaleAndTranslate(view.project(positions[i]))
			c.DrawLine(p1.X, p1.Y, p2.X, p2.Y)
			p1 = p2
		}
		p2 := view.scaleAndTranslate(view.project(arrival.NearestApproachPosition))
		c.DrawLine(p1.X, p1.Y, p2.X, p2.Y)
		c.Stroke()
	}
	return c.Image(), nil
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

func (scene Scene) PlotITD(X, Y int, arrivals []Arrival, window int) (image.Image, error) {
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
	}

	tmpdir, err := os.MkdirTemp("", "goroom")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpdir)

	itd, err := plotter.NewBarChart(v, vg.Points(3))
	if err != nil {
		return nil, err
	}
	p.Add(itd)
	if err := p.Save(font.Length(X), font.Length(Y), path.Join(tmpdir, "itd.png")); err != nil {
		return nil, err
	}
	f, err := os.Open(path.Join(tmpdir, "itd.png"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fmt.Println("Decoding PNG...")
	return png.Decode(f)
}
