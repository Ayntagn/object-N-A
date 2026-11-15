package ui

import (
	"math"
	"time"

	mygoui "github.com/egoist/mygo/ui"
)

const (
	graphTourInterval = 2600 * time.Millisecond
	graphTourMove     = 1100 * time.Millisecond
	graphTourReturn   = 800 * time.Millisecond
	graphShapeMove    = 900 * time.Millisecond
)

type graphPose struct {
	pan            mygoui.Point
	zoom, distance float64
	rotation       graphRotation
}

type graphTour struct {
	started, stopping time.Time
	step              int
	from, to, last    graphPose
}

func (v *graphCanvas) touring() bool { return !v.tour.started.IsZero() }

func (v *graphCanvas) cancelTour() {
	v.stopTour()
	v.tour = graphTour{}
}

func (v *graphCanvas) toggleTour() {
	if v.touring() {
		v.stopTour()
		return
	}
	if v.graph == nil || len(v.graph.Notes) == 0 {
		return
	}
	v.tour = graphTour{started: time.Now(), step: -1}
}

func (v *graphCanvas) stopTour() {
	if !v.touring() {
		return
	}
	tour := &v.tour
	tour.started = time.Time{}
	v.pan, v.zoom = mygoui.Point{}, 1
	v.paused, v.spinAt = false, time.Time{}
	if tour.step < 0 {
		return
	}
	if v.globe {
		rotation := tour.last.rotation
		yAxis := rotation.apply(graphPosition{y: 1})
		xAxis := rotation.apply(graphPosition{x: 1})
		zAxis := rotation.apply(graphPosition{z: 1})
		v.pitch = math.Atan2(yAxis.z, yAxis.y)
		v.yaw = math.Atan2(zAxis.x, xAxis.x)
	}
	tour.stopping, tour.from = time.Now(), tour.last
}

// EV-UI-097: one camera per host. A tour frame never invokes document navigation.
func (v *graphCanvas) tourFrame(now time.Time, base graphPose, options graphCanvasOptions) (graphPose, int) {
	tour := &v.tour
	if v.touring() {
		elapsed := max(0, now.Sub(tour.started))
		step := int(elapsed / graphTourInterval)
		order := v.layout.Order
		note := order[step%len(order)]
		if tour.step != step {
			tour.from = tour.last
			if tour.step < 0 {
				tour.from = base
			}
			tour.step, tour.to = step, base
			if options.Globe {
				rotation := graphAlignZ(v.spatial[note])
				tour.to = graphPose{
					zoom: 1, distance: 0.63 * options.cameraDistance(),
					rotation: graphRotation{-rotation.x, -rotation.y, -rotation.z, rotation.w},
				}
			} else {
				point := v.flat[note]
				if pin, ok := v.pins[v.graph.Notes[note].Path]; ok {
					point = pin
				}
				tour.to.zoom = 1.08 / 0.45
				if options.Compact {
					tour.to.zoom = 1 / 0.45
				}
				scale := graphFlatScale(v.size, options.Compact) * tour.to.zoom
				tour.to.pan = mygoui.Point{X: float32(-point.x * scale), Y: float32(-point.y * scale)}
			}
		}
		phase := float64(elapsed%graphTourInterval) / float64(graphTourMove)
		tour.last = blendGraphPose(tour.from, tour.to, graphEase(phase))
		return tour.last, note
	}
	if !tour.stopping.IsZero() {
		phase := float64(now.Sub(tour.stopping)) / float64(graphTourReturn)
		if phase < 1 {
			return blendGraphPose(tour.from, base, graphEase(phase)), -1
		}
		tour.stopping = time.Time{}
	}
	return base, -1
}

// SceneKit uses the standard ease-in/ease-out cubic for the tour transition.
func graphEase(progress float64) float64 {
	if progress <= 0 || progress >= 1 {
		return min(1, max(0, progress))
	}
	low, high := 0.0, 1.0
	for range 14 {
		t := (low + high) / 2
		u := 1 - t
		x := 3*u*u*t*0.42 + 3*u*t*t*0.58 + t*t*t
		if x < progress {
			low = t
		} else {
			high = t
		}
	}
	t := (low + high) / 2
	return t * t * (3 - 2*t)
}

func blendGraphPose(a, b graphPose, t float64) graphPose {
	u := float32(t)
	return graphPose{
		pan: mygoui.Point{
			X: a.pan.X + (b.pan.X-a.pan.X)*u,
			Y: a.pan.Y + (b.pan.Y-a.pan.Y)*u,
		},
		zoom:     a.zoom + (b.zoom-a.zoom)*t,
		distance: a.distance + (b.distance-a.distance)*t,
		rotation: a.rotation.blend(b.rotation, t),
	}
}

type graphRotation struct{ x, y, z, w float64 }

func graphEulerRotation(pitch, yaw float64) graphRotation {
	sx, cx := math.Sincos(pitch / 2)
	sy, cy := math.Sincos(yaw / 2)
	return graphRotation{sx * cy, cx * sy, sx * sy, cx * cy}
}

// graphAlignZ rotates the +Z pole of a cluster cap to its center.
func graphAlignZ(direction graphPosition) graphRotation {
	length := math.Sqrt(direction.x*direction.x + direction.y*direction.y + direction.z*direction.z)
	if length == 0 {
		return graphRotation{w: 1}
	}
	z := direction.z / length
	if z < -0.99999999 {
		return graphRotation{x: 1}
	}
	return (graphRotation{-direction.y / length, direction.x / length, 0, 1 + z}).unit()
}

func (q graphRotation) unit() graphRotation {
	length := math.Sqrt(q.x*q.x + q.y*q.y + q.z*q.z + q.w*q.w)
	return graphRotation{q.x / length, q.y / length, q.z / length, q.w / length}
}

func (q graphRotation) apply(p graphPosition) graphPosition {
	x := 2 * (q.y*p.z - q.z*p.y)
	y := 2 * (q.z*p.x - q.x*p.z)
	z := 2 * (q.x*p.y - q.y*p.x)
	return graphPosition{
		p.x + q.w*x + q.y*z - q.z*y,
		p.y + q.w*y + q.z*x - q.x*z,
		p.z + q.w*z + q.x*y - q.y*x,
	}
}

func (q graphRotation) blend(other graphRotation, t float64) graphRotation {
	dot := q.x*other.x + q.y*other.y + q.z*other.z + q.w*other.w
	if dot < 0 {
		other = graphRotation{-other.x, -other.y, -other.z, -other.w}
		dot = -dot
	}
	a, b := 1-t, t
	if dot < 0.9995 {
		angle := math.Acos(dot)
		sin := math.Sin(angle)
		a, b = math.Sin(a*angle)/sin, math.Sin(b*angle)/sin
	}
	return (graphRotation{a*q.x + b*other.x, a*q.y + b*other.y, a*q.z + b*other.z, a*q.w + b*other.w}).unit()
}
