package room

import (
	"fmt"
	"image"
	"image/png"
	"math"
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
	Scene Scene
	XSize int
	YSize int
	Plane Plane
	// These cache the values needed to scale and translate from the scene to the requested image size
	scale      float64
	xTranslate float64
	yTranslate float64
}

func (o View) project(v pt.Vector) Point2D {
	return To2D(o.Plane.Project(v))
}

type Scene struct {
	Sources           []Speaker
	ListeningPosition pt.Vector
	ListeningTriangle ListeningTriangle
	Room              *Room
}

func (scene Scene) BoundingBox(p Plane) (XMin, XMax, YMin, YMax float64) {
	// Sources
	for _, source := range scene.Sources {
		if source.Position.X < XMin {
			XMin = source.Position.X
		}
		if source.Position.X > XMax {
			XMax = source.Position.X
		}
		if source.Position.Y < YMin {
			YMin = source.Position.Y
		}
		if source.Position.Y > YMax {
			YMax = source.Position.Y
		}
	}
	// Listen Position
	if scene.ListeningPosition.X < XMin {
		XMin = scene.ListeningPosition.X
	}
	if scene.ListeningPosition.X > XMax {
		XMax = scene.ListeningPosition.X
	}
	if scene.ListeningPosition.Y < YMin {
		YMin = scene.ListeningPosition.Y
	}
	if scene.ListeningPosition.Y > YMax {
		YMax = scene.ListeningPosition.Y
	}
	// Room
	paths := p.MeshToPath(scene.Room.M)
	for _, path := range paths {
		pXMin, pXMax, pYMin, pYMax := path.BoundingBox()
		if pXMin < XMin {
			XMin = pXMin
		}
		if pXMax > XMax {
			XMax = pXMax
		}
		if pYMin < YMin {
			YMin = pYMin
		}
		if pYMax > YMax {
			YMax = pYMax
		}
	}

	return
}

func (view *View) computeScaleAndTranslation() {
	XMin, XMax, YMin, YMax := view.Scene.BoundingBox(view.Plane)
	view.xTranslate = -XMin
	view.yTranslate = -YMin
	XScale := float64(view.XSize) / (XMax - XMin)
	YScale := float64(view.YSize) / (YMax - YMin)
	view.scale = math.Min(XScale, YScale)
}

func (view *View) getScale() float64 {
	if view.scale == 0 {
		view.computeScaleAndTranslation()
	}
	return view.scale
}

func (view *View) getXTranslate() float64 {
	if view.scale == 0 {
		view.computeScaleAndTranslation()
	}
	return view.xTranslate
}

func (view *View) getYTranslate() float64 {
	if view.scale == 0 {
		view.computeScaleAndTranslation()
	}
	return view.yTranslate
}

func (o *View) translateAndScale(p Point2D) Point2D {
	return p.Translate(o.getXTranslate(), o.getYTranslate()).Scale(o.getScale())
}

func (view *View) PlotArrivals3D(arrivals []Arrival) (image.Image, error) {
	c := gg.NewContext(view.XSize, view.YSize)

	listenPos := view.translateAndScale(view.project(view.Scene.ListeningPosition))
	// Small circle represents the listening position
	c.DrawCircle(listenPos.X, listenPos.Y, 2) // last arg is radius
	c.Fill()
	// Large circle represents RFZ
	// TODO: need to get RFZ radius from trace params
	c.DrawCircle(listenPos.X, listenPos.Y, 50) // last arg is radius
	for _, source := range view.Scene.Sources {
		sourcePos := view.translateAndScale(view.project(source.Position))
		c.DrawCircle(sourcePos.X, sourcePos.Y, 2)
	}

	for _, lines := range view.Plane.MeshToPath(view.Scene.Room.M) {
		// lines.Scale(view)
		for i := 0; i < len(lines)-1; i++ {
			p1 := view.translateAndScale(lines[i])
			p2 := view.translateAndScale(lines[i+1])
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

		p1 := view.translateAndScale(view.project(arrival.Shot.ray.Origin))
		for i := 0; i < len(positions); i++ {
			c.SetLineWidth(arrival.Gain)
			p2 := view.translateAndScale(view.project(positions[i]))
			c.DrawLine(p1.X, p1.Y, p2.X, p2.Y)
			p1 = p2
		}
		p2 := view.translateAndScale(view.project(arrival.NearestApproachPosition))
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
