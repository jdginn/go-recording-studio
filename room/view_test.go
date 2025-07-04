package room

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fogleman/pt/pt"
)

func TestScaleView(t *testing.T) {
	assert := assert.New(t)

	view := View{
		Scene: Scene{
			Sources: []Speaker{
				{
					LoudSpeakerSpec: LoudSpeakerSpec{},
					Source:          Source{},
				},
			},
			ListeningPosition: V(-10, 10, 10), // XMin = -10
			ListeningTriangle: ListeningTriangle{
				ReferencePosition: V(5, 0, 5),
				ReferenceNormal:   V(1, 0, 0),
				DistFromFront:     0,
				DistFromCenter:    1,
				SourceHeight:      0.5,
				ListenHeight:      0.5,
			},
			Room: &Room{
				M: pt.NewCube(V(0, 0, 0), V(10, 10, 10), pt.Material{}).Mesh(), // YMin = 0, XMax = 10, YMax = 10
			},
		},
		XSize: 100,
		YSize: 100,
		Plane: MakePlane(V(0, 0, 0.5), V(0, 0, 1)),
	}

	view.computeScaleAndTranslation()

	// XSize = 20
	// YSize = 10
	assert.EqualValues(5, view.scale)
	assert.EqualValues(10, view.xTranslate)
	assert.EqualValues(0, view.yTranslate)
}
