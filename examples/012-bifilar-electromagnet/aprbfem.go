// -*- compile-command: "go run aprbfem.go"; -*-

// aprbfem creates an axial-plus-radial-bi-filar-electro-magnet
// STL file using Go. It is based on 30x30x132mm-vert.irmf.
//
// It generates the main metal (copper?) coil file plus a second
// file representing a dielectric (or support material) surrounding
// the metal (with suffic "-dielectric.stl" instead of ".stl".
//
// Usage:
//
//	go run aprbfem.go -h
//	go run aprbfem.go -out aprbfem.stl
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/gmlewis/go3d/vec3"
	"github.com/gmlewis/irmf-slicer/v3/stl"
)

var (
	filename = flag.String("out", "aprbfem.stl", "Output filename")
	dielGap  = flag.Float64("diel_gap", 0.05, "Gap between metal and dielectric (or support material)")
	dielPad  = flag.Float64("diel_pad", 0.3, "Padding between metal and outer edge of dielectric (or support material)")
	innerR   = flag.Float64("inner_radius", 3.0, "Inner radius in millimeters")
	leadLen  = flag.Float64("lead_len", 5.0, "Length of two external leads")
	numDivs  = flag.Int("num_divs", 36, "Number of divisions per rotation")
	numPairs = flag.Int("num_pairs", 11, "Number of coil pairs")
	numTurns = flag.Int("num_turns", 61, "Total number of turns per coil")
	wireGap  = flag.Float64("wire_gap", 0.15, "Gap between wires in millimeters")
	wireSize = flag.Float64("wire_size", 0.85, "Width of (square) wire in millimeters")
)

// TriWriter is a writer that writes STL triangles to a file.
type TriWriter interface {
	Write(t *stl.Tri) error
}

func main() {
	flag.Parse()

	if *dielGap*2 >= *wireGap {
		log.Fatal("-diel_gap (%v) must be less than half the -wire_gap (%v)", *dielGap, *wireGap)
	}

	w1, err := stl.New(*filename)
	if err != nil {
		log.Fatalf("stl.New: %v", err)
	}

	dielFilename := strings.TrimSuffix(*filename, ".stl") + "-dielectric.stl"
	w2, err := stl.New(dielFilename)
	if err != nil {
		log.Fatalf("stl.New: %v", err)
	}

	m := &arBifilarElectromagnet{
		numPairs:    *numPairs,
		innerRadius: *innerR,
		leadLen:     *leadLen,
		size:        *wireSize,
		singleGap:   *wireGap,
		numTurns:    *numTurns,
		w1:          &triWrapper{w: w1},
		w2:          &triWrapper{w: w2},

		lowerConnectors: map[string]*connector{},
	}

	m.render()

	if err := w1.Close(); err != nil {
		log.Fatalf("w1.Close: %v", err)
	}
	if err := w2.Close(); err != nil {
		log.Fatalf("w2.Close: %v", err)
	}

	log.Printf("Done.")
}

type arBifilarElectromagnet struct {
	// initializers:
	numPairs    int
	innerRadius float64
	leadLen     float64
	size        float64
	singleGap   float64
	numTurns    int
	w1, w2      triHelper

	// calculated:
	inc             float64
	connectorRadius float64
	doubleGap       float64
	height          float32
	dielFrontZ      float32
	dielBackZ       float32

	// used for collision detection
	coil2slope      float64
	coil2yIntercept float64

	// used to render top dielectric end cap
	debotP3uo *vec3.T
	debotP3ui *vec3.T
	debotP2uo *vec3.T
	debotP2ui *vec3.T
	debotP0uo *vec3.T
	debotP0ui *vec3.T
	debotP1uo *vec3.T
	debotP1ui *vec3.T

	lowerConnectors map[string]*connector
}

type connector struct {
	p1, p2, p3, p4         *vec3.T
	dep1, dep2, dep3, dep4 *vec3.T
}

func (m *arBifilarElectromagnet) render() {
	m.inc = math.Pi / float64(m.numPairs)
	m.connectorRadius = m.innerRadius + float64(m.numPairs)*(m.size+m.singleGap)
	m.doubleGap = m.size + 2*m.singleGap
	m.height = float32((m.size + m.singleGap) * float64((m.numTurns*2 + 1)))

	z0, adjz1 := m.calcWallParams()
	m.dielFrontZ = float32(z0 - 0.5*m.size - *dielGap - *dielPad)
	m.dielBackZ = m.height + float32(adjz1+0.5*m.size+*dielGap+*dielPad)

	for i := 1; i <= m.numPairs; i++ {
		m.coilPlusConnectorWires(1, i)
		m.coilPlusConnectorWires(2, i)
	}

	m.dielectricFrontEndAndWalls()
	m.dielectricBackEnd()
}

func (m *arBifilarElectromagnet) radiusOffset(coilNum int) float64 {
	return (m.size + m.singleGap) * float64(coilNum-1)
}

func (m *arBifilarElectromagnet) spacingAngle(coilNum int) float64 {
	// return float64(m.numPairs-4) * m.inc * math.Atan(2*m.radiusOffset(coilNum)/float64(m.numPairs-1))
	return 2 * float64(coilNum) / float64(m.numPairs-4)
}

func (m *arBifilarElectromagnet) coilRadius(coilNum int) float64 {
	return m.radiusOffset(coilNum) + m.innerRadius
}

func (m *arBifilarElectromagnet) endAngle(wireNum, coilNum int) float64 {
	radius := m.coilRadius(coilNum)
	spacingAngle := m.spacingAngle(coilNum)
	endAngle := float64(m.numTurns) * 2 * math.Pi

	nextCoilNum := coilNum + 1
	nextSpacingAngle := m.spacingAngle(nextCoilNum)
	if nextCoilNum > *numPairs {
		nextSpacingAngle = m.spacingAngle(1) + 2*math.Pi
	}

	// Account for the edge of the wire connector
	ro := radius + 0.5*m.size
	angleEnd := m.size / ro

	result := endAngle + nextSpacingAngle - math.Pi - spacingAngle - angleEnd

	// Special case for exit wire
	if coilNum == *numPairs && wireNum == 2 {
		result -= 0.5 * m.spacingAngle(1)
	}

	return result
}

func (m *arBifilarElectromagnet) coilPlusConnectorWires(wireNum, coilNum int) {
	radius := m.coilRadius(coilNum)
	spacingAngle := m.spacingAngle(coilNum)

	ri := radius - 0.5*m.size
	ro := radius + 0.5*m.size

	angle := 0.5 * m.size / ro // Start at the edge of the wire connector
	endAngle := m.endAngle(wireNum, coilNum) + angle
	delta := (endAngle - angle) / float64(*numDivs**numTurns)

	dielAngleDelta := *dielGap / ro
	dielAngle := angle + dielAngleDelta
	dielEndAngle := endAngle - dielAngleDelta
	dielDelta := (dielEndAngle - dielAngle) / float64(*numDivs**numTurns)

	// The first segment and the last segment are special cases because they connect
	// up to the wire segments that pair up the coils in the correct sequence.
	m.firstCoilWireSegment(wireNum, coilNum, spacingAngle, angle+spacingAngle, ri, ro)

	for i := 0; i < *numDivs**numTurns; i, angle, dielAngle = i+1, angle+delta, dielAngle+dielDelta {
		m.coilWireSegment(wireNum, coilNum, angle+spacingAngle, angle+delta+spacingAngle, ri, ro)
		m.coilDielSegment(wireNum, coilNum, dielAngle+spacingAngle, dielAngle+dielDelta+spacingAngle, ri, ro)
		if i == *numDivs**numTurns-1 {
			m.lastCoilWireSegment(wireNum, coilNum, angle+spacingAngle, angle+delta+spacingAngle, ri, ro)
		}
	}
}

type triHelper interface {
	writeTri(normal, v1, v2, v3 *vec3.T)
}

type triWrapper struct {
	w TriWriter
}

func (t *triWrapper) writeTri(normal, v1, v2, v3 *vec3.T) {
	t.w.Write(&stl.Tri{
		N:  [3]float32{normal[0], normal[1], normal[2]},
		V1: [3]float32{v1[0], v1[1], v1[2]},
		V2: [3]float32{v2[0], v2[1], v2[2]},
		V3: [3]float32{v3[0], v3[1], v3[2]},
	})
}

func cp(v *vec3.T) *vec3.T { return &vec3.T{v[0], v[1], v[2]} }

func (m *arBifilarElectromagnet) metalQuad(v1, v2, v3, v4 *vec3.T) (*vec3.T, *vec3.T) {
	v31 := cp(v3).Sub(v1)
	n1 := vec3.Cross(cp(v2).Sub(v1), v31)
	n1.Normalize()
	m.w1.writeTri(&n1, v1, v2, v3)
	n2 := vec3.Cross(v31, cp(v4).Sub(v1))
	n2.Normalize()
	m.w1.writeTri(&n2, v1, v3, v4)
	return &n1, &n2
}

func (m *arBifilarElectromagnet) dielQuad(v1, v2, v3, v4 *vec3.T) (*vec3.T, *vec3.T) {
	v31 := cp(v3).Sub(v1)
	n1 := vec3.Cross(v31, cp(v2).Sub(v1))
	n1.Normalize()
	m.w2.writeTri(&n1, v1, v3, v2) // NOTE: v2 and v3 are switched here to reverse the normal!
	n2 := vec3.Cross(cp(v4).Sub(v1), v31)
	n2.Normalize()
	m.w2.writeTri(&n2, v1, v4, v3) // NOTE: v3 and v4 are switched here to reverse the normal!
	return &n1, &n2
}

// dielTri creates a triangle without the normals being reversed (i.e. standard ordering).
func (m *arBifilarElectromagnet) dielTri(v1, v2, v3 *vec3.T) {
	v31 := cp(v3).Sub(v1)
	n1 := vec3.Cross(cp(v2).Sub(v1), v31)
	n1.Normalize()
	m.w2.writeTri(&n1, v1, v2, v3)
}

type pFunc func(r, a, z float64) *vec3.T

func (m *arBifilarElectromagnet) calcAnglesZsAndPs(wireNum int, origA1, origA2 float64) (a1, a2, z1, z2 float64, pu, pd pFunc) {
	a1, a2 = origA1, origA2
	z1 = (m.size + m.singleGap) * a1 / math.Pi
	z2 = (m.size + m.singleGap) * a2 / math.Pi
	if wireNum == 2 {
		a1 += math.Pi
		a2 += math.Pi
	}

	pu = func(r, a, z float64) *vec3.T {
		return &vec3.T{float32(r * math.Cos(a)), float32(r * math.Sin(a)), float32(z - 0.5*m.size)}
	}
	pd = func(r, a, z float64) *vec3.T {
		return &vec3.T{float32(r * math.Cos(a)), float32(r * math.Sin(a)), float32(z + 0.5*m.size)}
	}

	return a1, a2, z1, z2, pu, pd
}

func (m *arBifilarElectromagnet) calcFirstCoilParams(a1, origA1, ro float64) (a0, adja1, z0, adjz1 float64) {
	da := m.size / ro
	a0 = a1 - 0.5*da
	adja1 = a1 + 0.5*da
	z0 = (m.size + m.singleGap) * (origA1 - 0.5*da) / math.Pi
	adjz1 = (m.size + m.singleGap) * (origA1 + 0.5*da) / math.Pi
	return a0, adja1, z0, adjz1
}

func (m *arBifilarElectromagnet) firstCoilWireSegment(wireNum, coilNum int, origA1, origA2, ri, ro float64) {
	a1, _, _, _, pu, pd := m.calcAnglesZsAndPs(wireNum, origA1, origA2)

	// a1 is the center of the connector
	a0, adja1, z0, adjz1 := m.calcFirstCoilParams(a1, origA1, ro)

	p0uo := pu(ro, a0, z0)
	p0ui := pu(ri, a0, z0)
	p0do := pd(ro, a0, z0)
	p0di := pd(ri, a0, z0)

	adjP1uo := pu(ro, adja1, adjz1)
	adjP1ui := pu(ri, adja1, adjz1)
	adjP1do := pd(ro, adja1, adjz1)
	adjP1di := pd(ri, adja1, adjz1)

	n := vec3.Cross(cp(p0di).Sub(p0do), cp(p0ui).Sub(p0do))
	n.Normalize()

	m.metalQuad(p0do, p0di, p0ui, p0uo)       // end-cap
	m.metalQuad(p0uo, p0ui, adjP1ui, adjP1uo) // upward
	m.metalQuad(p0ui, p0di, adjP1di, adjP1ui) // inner
	m.metalQuad(p0do, adjP1do, adjP1di, p0di) // downward

	// dielectric
	aDelta := *dielGap / ro
	dep0uo := pu(ro+*dielGap, a0-aDelta, z0-*dielGap)
	dep0ui := pu(ri-*dielGap, a0-aDelta, z0-*dielGap)
	dep0do := pd(ro+*dielGap, a0-aDelta, z0+*dielGap)
	dep0di := pd(ri-*dielGap, a0-aDelta, z0+*dielGap)
	deadjP1uo := pu(ro+*dielGap, adja1+aDelta, adjz1-*dielGap)
	deadjP1ui := pu(ri-*dielGap, adja1+aDelta, adjz1-*dielGap)
	deadjP1do := pd(ro+*dielGap, adja1+aDelta, adjz1+*dielGap)
	deadjP1di := pd(ri-*dielGap, adja1+aDelta, adjz1+*dielGap)
	m.dielQuad(dep0do, dep0di, dep0ui, dep0uo)       // end-cap
	m.dielQuad(dep0uo, dep0ui, deadjP1ui, deadjP1uo) // upward
	m.dielQuad(dep0ui, dep0di, deadjP1di, deadjP1ui) // inner
	m.dielQuad(dep0do, deadjP1do, deadjP1di, dep0di) // downward

	ni01 := vec3.T{float32(math.Cos(a1 + math.Pi)), float32(math.Sin(a1 + math.Pi)), 0}

	vlen := m.connectorRadius + 0.5*m.size - ro

	outP0uo := cp(&ni01).Scale(-float32(vlen)).Add(p0uo)
	outP0ui := cp(&ni01).Scale(-float32(vlen)).Add(p0ui)
	outP0do := cp(&ni01).Scale(-float32(vlen)).Add(p0do)
	outP0di := cp(&ni01).Scale(-float32(vlen)).Add(p0di)
	outP1uo := cp(&ni01).Scale(-float32(vlen)).Add(adjP1uo)
	outP1ui := cp(&ni01).Scale(-float32(vlen)).Add(adjP1ui)
	outP1do := cp(&ni01).Scale(-float32(vlen)).Add(adjP1do)
	outP1di := cp(&ni01).Scale(-float32(vlen)).Add(adjP1di)

	zu := 0.5 * (outP0ui[2] + outP1ui[2])
	zd := 0.5 * (outP0di[2] + outP1di[2])
	outP0uo[2] = zu
	outP0ui[2] = zu
	outP0do[2] = zd
	outP0di[2] = zd
	outP1uo[2] = zu
	outP1ui[2] = zu
	outP1do[2] = zd
	outP1di[2] = zd

	m.metalQuad(outP0do, outP0di, outP0ui, outP0uo) // end-cap
	m.metalQuad(outP0di, p0do, p0uo, outP0ui)       // end-cap connector
	m.metalQuad(outP0uo, outP1uo, outP1do, outP0do) // outer
	m.metalQuad(outP0ui, p0uo, adjP1uo, outP1ui)    // upward connector
	m.metalQuad(outP0uo, outP0ui, outP1ui, outP1uo) // upward
	m.metalQuad(outP0di, outP1di, adjP1do, p0do)    // downward
	m.metalQuad(outP1do, outP1uo, outP1ui, outP1di) // backface
	m.metalQuad(outP1di, outP1ui, adjP1uo, adjP1do) // backface connector

	// dielectric
	deoutP0uo := cp(&ni01).Scale(-float32(vlen)).Add(dep0uo)
	deoutP0ui := cp(&ni01).Scale(-float32(vlen)).Add(dep0ui)
	deoutP0do := cp(&ni01).Scale(-float32(vlen)).Add(dep0do)
	deoutP0di := cp(&ni01).Scale(-float32(vlen)).Add(dep0di)
	deoutP1uo := cp(&ni01).Scale(-float32(vlen)).Add(deadjP1uo)
	deoutP1ui := cp(&ni01).Scale(-float32(vlen)).Add(deadjP1ui)
	deoutP1do := cp(&ni01).Scale(-float32(vlen)).Add(deadjP1do)
	deoutP1di := cp(&ni01).Scale(-float32(vlen)).Add(deadjP1di)
	m.dielQuad(deoutP0do, deoutP0di, deoutP0ui, deoutP0uo) // end-cap
	m.dielQuad(deoutP0di, dep0do, dep0uo, deoutP0ui)       // end-cap connector
	m.dielQuad(deoutP0uo, deoutP1uo, deoutP1do, deoutP0do) // outer
	m.dielQuad(deoutP0ui, dep0uo, deadjP1uo, deoutP1ui)    // upward connector
	m.dielQuad(deoutP0uo, deoutP0ui, deoutP1ui, deoutP1uo) // upward
	m.dielQuad(deoutP0di, deoutP1di, deadjP1do, dep0do)    // downward
	m.dielQuad(deoutP1do, deoutP1uo, deoutP1ui, deoutP1di) // backface
	m.dielQuad(deoutP1di, deoutP1ui, deadjP1uo, deadjP1do) // backface connector

	h := m.height
	dielH := h
	if coilNum == 1 && wireNum == 1 {
		h += float32(*leadLen + 0.5*m.size + *dielPad) // exit wire height
	}

	botP0uo := cp(&vec3.UnitZ).Scale(h).Add(outP0uo)
	botP0ui := cp(&vec3.UnitZ).Scale(h).Add(outP0ui)
	botP0do := cp(&vec3.UnitZ).Scale(h).Add(outP0do)
	botP0di := cp(&vec3.UnitZ).Scale(h).Add(outP0di)
	botP1uo := cp(&vec3.UnitZ).Scale(h).Add(outP1uo)
	botP1ui := cp(&vec3.UnitZ).Scale(h).Add(outP1ui)
	botP1do := cp(&vec3.UnitZ).Scale(h).Add(outP1do)
	botP1di := cp(&vec3.UnitZ).Scale(h).Add(outP1di)
	if coilNum == 1 && wireNum == 1 {
		botP0uo[2] = h
		botP0ui[2] = h
		botP0do[2] = h
		botP0di[2] = h
		botP1uo[2] = h
		botP1ui[2] = h
		botP1do[2] = h
		botP1di[2] = h
	}

	// axial connector
	m.metalQuad(botP0uo, botP0ui, outP0di, outP0do) // forward (was end-cap)
	m.metalQuad(outP0do, outP1do, botP1uo, botP0uo) // outer
	m.metalQuad(botP1uo, outP1do, outP1di, botP1ui) // backface
	m.metalQuad(botP0ui, botP1ui, outP1di, outP0di) // inner

	// dielectrics
	debotP0uo := cp(&vec3.UnitZ).Scale(dielH).Add(deoutP0uo)
	debotP0ui := cp(&vec3.UnitZ).Scale(dielH).Add(deoutP0ui)
	debotP0do := cp(&vec3.UnitZ).Scale(dielH).Add(deoutP0do)
	debotP0di := cp(&vec3.UnitZ).Scale(dielH).Add(deoutP0di)
	debotP1uo := cp(&vec3.UnitZ).Scale(dielH).Add(deoutP1uo)
	debotP1ui := cp(&vec3.UnitZ).Scale(dielH).Add(deoutP1ui)
	debotP1do := cp(&vec3.UnitZ).Scale(dielH).Add(deoutP1do)
	debotP1di := cp(&vec3.UnitZ).Scale(dielH).Add(deoutP1di)
	if coilNum == 1 && wireNum == 1 { // flat dielectric exit
		debotP0uo[2] = m.dielBackZ
		debotP0ui[2] = m.dielBackZ
		debotP0do[2] = m.dielBackZ
		debotP0di[2] = m.dielBackZ
		debotP1uo[2] = m.dielBackZ
		debotP1ui[2] = m.dielBackZ
		debotP1do[2] = m.dielBackZ
		debotP1di[2] = m.dielBackZ

		m.debotP0uo = debotP0uo
		m.debotP0ui = debotP0ui
		m.debotP1uo = debotP1uo
		m.debotP1ui = debotP1ui
	}
	m.dielQuad(debotP0uo, debotP0ui, deoutP0di, deoutP0do) // forward (was end-cap)
	m.dielQuad(deoutP0do, deoutP1do, debotP1uo, debotP0uo) // outer
	m.dielQuad(debotP1uo, deoutP1do, deoutP1di, debotP1ui) // backface
	m.dielQuad(debotP0ui, debotP1ui, deoutP1di, deoutP0di) // inner

	if coilNum == 1 && wireNum == 1 {
		m.metalQuad(botP1uo, botP1ui, botP0ui, botP0uo) // end cap of exit wire
		return
	}

	// angle connector
	m.metalQuad(botP0do, botP0di, botP0ui, botP0uo) // forward (was end-cap)
	m.metalQuad(botP1do, botP0do, botP0uo, botP1uo) // outer
	m.metalQuad(botP1do, botP1uo, botP1ui, botP1di) // backface
	m.metalQuad(botP1do, botP1di, botP0di, botP0do) // end cap

	// dielectric
	m.dielQuad(debotP0do, debotP0di, debotP0ui, debotP0uo) // forward (was end-cap)
	m.dielQuad(debotP1do, debotP0do, debotP0uo, debotP1uo) // outer
	m.dielQuad(debotP1do, debotP1uo, debotP1ui, debotP1di) // backface
	m.dielQuad(debotP1do, debotP1di, debotP0di, debotP0do) // end cap

	// connector to inner start of next coil
	nextRing := coilNum - 1
	if nextRing < 1 {
		nextRing = *numPairs
	}
	nextRadius := m.coilRadius(nextRing)

	conlen := m.connectorRadius - nextRadius
	conP0uo := cp(&ni01).Scale(float32(conlen)).Add(outP0uo)
	conP0do := cp(&ni01).Scale(float32(conlen)).Add(outP0do)
	conP1uo := cp(&ni01).Scale(float32(conlen)).Add(outP1uo)
	conP1do := cp(&ni01).Scale(float32(conlen)).Add(outP1do)
	conP0uo[2] = botP0uo[2]
	conP0do[2] = botP0do[2]
	conP1uo[2] = botP1uo[2]
	conP1do[2] = botP1do[2]

	// radial connector
	m.metalQuad(botP0do, botP0di, botP0ui, botP0uo) // end-cap
	m.metalQuad(botP0di, conP0do, conP0uo, botP0ui) // end-cap connector
	m.metalQuad(botP0uo, botP1uo, botP1do, botP0do) // outer
	m.metalQuad(botP0ui, conP0uo, conP1uo, botP1ui) // upward connector
	m.metalQuad(botP0uo, botP0ui, botP1ui, botP1uo) // upward
	m.metalQuad(botP0di, botP1di, conP1do, conP0do) // downward
	m.metalQuad(botP1do, botP1uo, botP1ui, botP1di) // backface
	m.metalQuad(botP1di, botP1ui, conP1uo, conP1do) // backface connector

	// dielectric
	deconP0uo := cp(&ni01).Scale(float32(conlen)).Add(deoutP0uo)
	deconP0do := cp(&ni01).Scale(float32(conlen)).Add(deoutP0do)
	deconP1uo := cp(&ni01).Scale(float32(conlen)).Add(deoutP1uo)
	deconP1do := cp(&ni01).Scale(float32(conlen)).Add(deoutP1do)
	deconP0uo[2] = debotP0uo[2]
	deconP0do[2] = debotP0do[2]
	deconP1uo[2] = debotP1uo[2]
	deconP1do[2] = debotP1do[2]
	m.dielQuad(debotP0do, debotP0di, debotP0ui, debotP0uo) // end-cap
	m.dielQuad(debotP0di, deconP0do, deconP0uo, debotP0ui) // end-cap connector
	m.dielQuad(debotP0uo, debotP1uo, debotP1do, debotP0do) // outer
	m.dielQuad(debotP0ui, deconP0uo, deconP1uo, debotP1ui) // upward connector
	m.dielQuad(debotP0uo, debotP0ui, debotP1ui, debotP1uo) // upward
	m.dielQuad(debotP0di, debotP1di, deconP1do, deconP0do) // downward
	m.dielQuad(debotP1do, debotP1uo, debotP1ui, debotP1di) // backface
	m.dielQuad(debotP1di, debotP1ui, deconP1uo, deconP1do) // backface connector

	extP0uo := cp(&ni01).Scale(float32(m.size)).Add(conP0uo)
	extP0do := cp(&ni01).Scale(float32(m.size)).Add(conP0do)
	extP1uo := cp(&ni01).Scale(float32(m.size)).Add(conP1uo)
	extP1do := cp(&ni01).Scale(float32(m.size)).Add(conP1do)

	// Check for possible wire intersection at one of the closest locations between coil 2 and coil 3
	if coilNum == 3 && wireNum == 1 {
		denom := math.Sqrt(m.coil2slope*m.coil2slope + 1)
		if denom == 0 {
			log.Fatal("-inner_radius too small for other params; wires would cross")
		}
		d := (m.coil2slope*float64(extP0uo[0]) - float64(extP0uo[1]) + m.coil2yIntercept) / denom
		if d >= 0 {
			log.Fatal("-inner_radius too small for other params; wires would cross")
		}
		if -d < *wireGap {
			log.Printf("WARNING: wire gap will be %0.3fmm in some places! Best to increase -inner_radius.", -d)
		}
	}
	if coilNum == 2 && wireNum == 1 {
		m.coil2slope = float64(botP1ui[1]-conP1uo[1]) / float64(botP1ui[0]-conP1uo[0])
		m.coil2yIntercept = float64(botP1ui[1]) - m.coil2slope*float64(botP1ui[0])
	}

	m.metalQuad(conP0do, extP0do, extP0uo, conP0uo) // frontface connector
	m.metalQuad(conP0do, conP1do, extP1do, extP0do) // downward connector
	m.metalQuad(conP1do, conP1uo, extP1uo, extP1do) // backface connector
	m.metalQuad(extP0do, extP1do, extP1uo, extP0uo) // end-cap connector

	// dielectric
	deextP0uo := cp(&ni01).Scale(float32(m.size + 2**dielGap)).Add(deconP0uo)
	deextP0do := cp(&ni01).Scale(float32(m.size + 2**dielGap)).Add(deconP0do)
	deextP1uo := cp(&ni01).Scale(float32(m.size + 2**dielGap)).Add(deconP1uo)
	deextP1do := cp(&ni01).Scale(float32(m.size + 2**dielGap)).Add(deconP1do)
	m.dielQuad(deconP0do, deextP0do, deextP0uo, deconP0uo) // frontface connector
	m.dielQuad(deconP0do, deconP1do, deextP1do, deextP0do) // downward connector
	m.dielQuad(deconP1do, deconP1uo, deextP1uo, deextP1do) // backface connector
	m.dielQuad(deextP0do, deextP1do, deextP1uo, deextP0uo) // end-cap connector

	key := fmt.Sprintf("%v,%v", 3-wireNum, coilNum-1)
	if lc, ok := m.lowerConnectors[key]; ok {
		m.metalQuad(conP0uo, extP0uo, lc.p1, lc.p2)
		m.metalQuad(extP1uo, conP1uo, lc.p3, lc.p4)
		m.metalQuad(conP1uo, conP0uo, lc.p2, lc.p3)
		m.metalQuad(extP0uo, extP1uo, lc.p4, lc.p1)

		// dielectric
		m.dielQuad(deconP0uo, deextP0uo, lc.dep1, lc.dep2)
		m.dielQuad(deextP1uo, deconP1uo, lc.dep3, lc.dep4)
		m.dielQuad(deconP1uo, deconP0uo, lc.dep2, lc.dep3)
		m.dielQuad(deextP0uo, deextP1uo, lc.dep4, lc.dep1)
	}
}

func (m *arBifilarElectromagnet) coilWireSegment(wireNum, coilNum int, origA1, origA2, ri, ro float64) {
	a1, a2, z1, z2, pu, pd := m.calcAnglesZsAndPs(wireNum, origA1, origA2)

	p1uo := pu(ro, a1, z1)
	p1ui := pu(ri, a1, z1)
	p1do := pd(ro, a1, z1)
	p1di := pd(ri, a1, z1)
	p2uo := pu(ro, a2, z2)
	p2ui := pu(ri, a2, z2)
	p2do := pd(ro, a2, z2)
	p2di := pd(ri, a2, z2)

	nu := vec3.Cross(cp(p2ui).Sub(p1uo), cp(p2uo).Sub(p1uo))
	nu.Normalize()
	nd := vec3.Cross(cp(p2do).Sub(p1do), cp(p2di).Sub(p1do))
	nd.Normalize()

	m.metalQuad(p1uo, p2uo, p2do, p1do) // outer-facing
	m.metalQuad(p1uo, p1ui, p2ui, p2uo) // upward-facing
	m.metalQuad(p1ui, p1di, p2di, p2ui) // inner-facing
	m.metalQuad(p1do, p2do, p2di, p1di) // downward-facing
}

func (m *arBifilarElectromagnet) coilDielSegment(wireNum, coilNum int, origA1, origA2, ri, ro float64) {
	a1, a2, z1, z2, pu, pd := m.calcAnglesZsAndPs(wireNum, origA1, origA2)

	// dielectric
	dep1uo := pu(ro+*dielGap, a1, z1-*dielGap)
	dep1ui := pu(ri-*dielGap, a1, z1-*dielGap)
	dep1do := pd(ro+*dielGap, a1, z1+*dielGap)
	dep1di := pd(ri-*dielGap, a1, z1+*dielGap)
	dep2uo := pu(ro+*dielGap, a2, z2-*dielGap)
	dep2ui := pu(ri-*dielGap, a2, z2-*dielGap)
	dep2do := pd(ro+*dielGap, a2, z2+*dielGap)
	dep2di := pd(ri-*dielGap, a2, z2+*dielGap)
	m.dielQuad(dep1uo, dep2uo, dep2do, dep1do) // outer-facing
	m.dielQuad(dep1uo, dep1ui, dep2ui, dep2uo) // upward-facing
	m.dielQuad(dep1ui, dep1di, dep2di, dep2ui) // inner-facing
	m.dielQuad(dep1do, dep2do, dep2di, dep1di) // downward-facing
}

func (m *arBifilarElectromagnet) calcLastCoilParams(a2, origA2, ro float64) (a3, z3 float64) {
	da := m.size / ro
	a3 = a2 + da
	z3 = (m.size + m.singleGap) * (origA2 + da) / math.Pi
	return a3, z3
}

func (m *arBifilarElectromagnet) lastCoilWireSegment(wireNum, coilNum int, origA1, origA2, ri, ro float64) {
	_, a2, _, z2, pu, pd := m.calcAnglesZsAndPs(wireNum, origA1, origA2)

	p2uo := pu(ro, a2, z2)
	p2ui := pu(ri, a2, z2)
	p2do := pd(ro, a2, z2)
	p2di := pd(ri, a2, z2)

	aDelta := *dielGap / ro
	dep2uo := pu(ro+*dielGap, a2-aDelta, z2-*dielGap)
	dep2ui := pu(ri-*dielGap, a2-aDelta, z2-*dielGap)
	dep2do := pd(ro+*dielGap, a2-aDelta, z2+*dielGap)
	dep2di := pd(ri-*dielGap, a2-aDelta, z2+*dielGap)

	// a2 is the end of the spiral
	a3, z3 := m.calcLastCoilParams(a2, origA2, ro)

	p3uo := pu(ro, a3, z3)
	p3ui := pu(ri, a3, z3)
	p3do := pd(ro, a3, z3)
	p3di := pd(ri, a3, z3)

	dep3uo := pu(ro+*dielGap, a3+aDelta, z3-*dielGap)
	dep3ui := pu(ri-*dielGap, a3+aDelta, z3-*dielGap)
	dep3do := pd(ro+*dielGap, a3+aDelta, z3+*dielGap)
	dep3di := pd(ri-*dielGap, a3+aDelta, z3+*dielGap)

	if coilNum == *numPairs && wireNum == 2 { // exit wire
		m.metalQuad(p3di, p3do, p3uo, p3ui) // end-cap
		m.metalQuad(p2uo, p3uo, p3do, p2do) // outer
		m.metalQuad(p2ui, p2di, p3di, p3ui) // inner
		m.metalQuad(p3ui, p3uo, p2uo, p2ui) // upward

		// dielectric
		m.dielQuad(dep3di, dep3do, dep3uo, dep3ui) // end-cap
		m.dielQuad(dep2uo, dep3uo, dep3do, dep2do) // outer
		m.dielQuad(dep2ui, dep2di, dep3di, dep3ui) // inner
		m.dielQuad(dep3ui, dep3uo, dep2uo, dep2ui) // upward

		h := m.height + float32(*leadLen+0.5*m.size+*dielPad) // exit wire height
		botP3uo := cp(&vec3.UnitZ).Add(p3uo)
		botP3ui := cp(&vec3.UnitZ).Add(p3ui)
		botP2uo := cp(&vec3.UnitZ).Add(p2uo)
		botP2ui := cp(&vec3.UnitZ).Add(p2ui)
		botP3uo[2] = h
		botP3ui[2] = h
		botP2uo[2] = h
		botP2ui[2] = h

		m.metalQuad(botP3ui, botP3uo, p3do, p3di) // forward
		m.metalQuad(botP3uo, botP2uo, p2do, p3do) // outer
		m.metalQuad(botP2uo, botP2ui, p2di, p2do) // backface
		m.metalQuad(botP2ui, botP3ui, p3di, p2di) // inner

		// dielectric
		m.debotP3uo = cp(&vec3.UnitZ).Add(dep3uo)
		m.debotP3ui = cp(&vec3.UnitZ).Add(dep3ui)
		m.debotP2uo = cp(&vec3.UnitZ).Add(dep2uo)
		m.debotP2ui = cp(&vec3.UnitZ).Add(dep2ui)
		m.debotP3uo[2] = m.dielBackZ
		m.debotP3ui[2] = m.dielBackZ
		m.debotP2uo[2] = m.dielBackZ
		m.debotP2ui[2] = m.dielBackZ
		m.dielQuad(m.debotP3ui, m.debotP3uo, dep3do, dep3di) // forward
		m.dielQuad(m.debotP3uo, m.debotP2uo, dep2do, dep3do) // outer
		m.dielQuad(m.debotP2uo, m.debotP2ui, dep2di, dep2do) // backface
		m.dielQuad(m.debotP2ui, m.debotP3ui, dep3di, dep2di) // inner

		m.metalQuad(botP3uo, botP3ui, botP2ui, botP2uo) // end cap of exit wire
		return
	}

	if coilNum == *numPairs && wireNum == 1 { // special loop-back case
		m.metalQuad(p3di, p3do, p3uo, p3ui) // end-cap
		m.metalQuad(p2ui, p2di, p3di, p3ui) // inner
		m.metalQuad(p3ui, p3uo, p2uo, p2ui) // upward
		m.metalQuad(p2di, p2do, p3do, p3di) // downward

		// dielectric
		m.dielQuad(dep3di, dep3do, dep3uo, dep3ui) // end-cap
		m.dielQuad(dep2ui, dep2di, dep3di, dep3ui) // inner
		m.dielQuad(dep3ui, dep3uo, dep2uo, dep2ui) // upward
		m.dielQuad(dep2di, dep2do, dep3do, dep3di) // downward
		return
	}

	m.metalQuad(p3di, p3do, p3uo, p3ui) // end-cap
	m.metalQuad(p2uo, p3uo, p3do, p2do) // outer
	m.metalQuad(p2ui, p2di, p3di, p3ui) // inner
	m.metalQuad(p3ui, p3uo, p2uo, p2ui) // upward

	// dielectric
	m.dielQuad(dep3di, dep3do, dep3uo, dep3ui) // end-cap
	m.dielQuad(dep2uo, dep3uo, dep3do, dep2do) // outer
	m.dielQuad(dep2ui, dep2di, dep3di, dep3ui) // inner
	m.dielQuad(dep3ui, dep3uo, dep2uo, dep2ui) // upward

	key := fmt.Sprintf("%v,%v", wireNum, coilNum)
	lc := &connector{
		p1: p2di,
		p2: p2do,
		p3: p3do,
		p4: p3di,

		dep1: dep2di,
		dep2: dep2do,
		dep3: dep3do,
		dep4: dep3di,
	}
	m.lowerConnectors[key] = lc
}

func (m *arBifilarElectromagnet) calcWallParams() (z0, adjz1 float64) {
	const wireNum = 1
	const coilNumFront = 1
	radius := m.coilRadius(coilNumFront)
	origA1 := m.spacingAngle(coilNumFront)
	ro := radius + 0.5*m.size
	origA2 := origA1 + 0.5*m.size/ro
	a1, _, _, _, _, _ := m.calcAnglesZsAndPs(wireNum, origA1, origA2)
	_, _, z0, _ = m.calcFirstCoilParams(a1, origA1, ro)

	coilNumBack := *numPairs
	radiusBack := m.coilRadius(coilNumBack)
	origA1Back := m.spacingAngle(coilNumBack)
	roBack := radiusBack + 0.5*m.size
	origA2Back := origA1Back + 0.5*m.size/roBack
	a1Back, _, _, _, _, _ := m.calcAnglesZsAndPs(wireNum, origA1Back, origA2Back)
	_, _, _, adjz1 = m.calcFirstCoilParams(a1Back, origA1Back, roBack)

	return z0, adjz1
}

func (m *arBifilarElectromagnet) dielectricFrontEndAndWalls() {
	defront := &vec3.T{0, 0, m.dielFrontZ}
	deback := &vec3.T{0, 0, m.dielBackZ}
	angle := 0.0
	delta := 2 * math.Pi / float64(*numDivs)
	r := m.connectorRadius + 0.5*m.size + *dielPad

	for i := 0; i < *numDivs; i++ {
		nextAngle := angle + delta
		if i == *numDivs-1 {
			nextAngle = 0
		}
		x1 := float32(r * math.Cos(angle))
		y1 := float32(r * math.Sin(angle))
		x2 := float32(r * math.Cos(nextAngle))
		y2 := float32(r * math.Sin(nextAngle))
		angle = nextAngle

		// end cap
		de1 := &vec3.T{x1, y1, defront[2]}
		de2 := &vec3.T{x2, y2, defront[2]}
		m.dielTri(defront, de2, de1)

		// wall
		de3 := &vec3.T{x1, y1, deback[2]}
		de4 := &vec3.T{x2, y2, deback[2]}
		m.dielTri(de1, de2, de4)
		m.dielTri(de1, de4, de3)
	}
}

func (m *arBifilarElectromagnet) dielectricBackEnd() {
	deback := &vec3.T{0, 0, m.dielBackZ}
	angle, nextAngle := 0.0, 0.0
	delta := 2 * math.Pi / float64(*numDivs)
	r := m.connectorRadius + 0.5*m.size + *dielPad
	p0a := math.Atan2(float64(m.debotP0uo[1]), float64(m.debotP0uo[0]))
	p1a := math.Atan2(float64(m.debotP1uo[1]), float64(m.debotP1uo[0]))
	p2a := math.Atan2(float64(m.debotP2uo[1]), float64(m.debotP2uo[0]))
	p3a := math.Atan2(float64(m.debotP3uo[1]), float64(m.debotP3uo[0]))
	p0ri := math.Sqrt(float64(m.debotP0ui[0]*m.debotP0ui[0] + m.debotP0ui[1]*m.debotP0ui[1]))
	p0ro := math.Sqrt(float64(m.debotP0uo[0]*m.debotP0uo[0] + m.debotP0uo[1]*m.debotP0uo[1]))
	p1ri := math.Sqrt(float64(m.debotP1ui[0]*m.debotP1ui[0] + m.debotP1ui[1]*m.debotP1ui[1]))
	p1ro := math.Sqrt(float64(m.debotP1uo[0]*m.debotP1uo[0] + m.debotP1uo[1]*m.debotP1uo[1]))
	p2ri := math.Sqrt(float64(m.debotP2ui[0]*m.debotP2ui[0] + m.debotP2ui[1]*m.debotP2ui[1]))
	p2ro := math.Sqrt(float64(m.debotP2uo[0]*m.debotP2uo[0] + m.debotP2uo[1]*m.debotP2uo[1]))
	p3ri := math.Sqrt(float64(m.debotP3ui[0]*m.debotP3ui[0] + m.debotP3ui[1]*m.debotP3ui[1]))
	p3ro := math.Sqrt(float64(m.debotP3uo[0]*m.debotP3uo[0] + m.debotP3uo[1]*m.debotP3uo[1]))

	for i := 0; i < *numDivs; i, angle = i+1, nextAngle {
		nextAngle = angle + delta
		if i == *numDivs-1 {
			nextAngle = 0
		}

		x1 := float32(r * math.Cos(angle))
		y1 := float32(r * math.Sin(angle))
		x2 := float32(r * math.Cos(nextAngle))
		y2 := float32(r * math.Sin(nextAngle))
		de1 := &vec3.T{x1, y1, deback[2]}
		de2 := &vec3.T{x2, y2, deback[2]}

		// Check for collision with the first exit wire.
		if i+1 != *numDivs {
			if angle < p0a && nextAngle > p1a {
				m.dielBackFullySurrounded(p0ri, p0ro, p1ri, p1ro,
					angle, nextAngle, p1a, deback, de1, de2,
					m.debotP0ui, m.debotP0uo, m.debotP1ui, m.debotP1uo)
				continue
			}

			if angle < p0a && nextAngle > p0a {
				m.dielBackStraddleRight(p0ri, p0ro, angle, nextAngle, deback, de1, de2, m.debotP0ui, m.debotP0uo)
				continue
			}

			if angle > p0a && nextAngle < p1a {
				m.dielBackBetweenCenter(p0ri, p0ro, angle, nextAngle, deback, de1, de2)
				continue
			}

			if angle < p1a && nextAngle > p1a {
				m.dielBackStraddleLeft(p1ri, p1ro, angle, nextAngle, deback, de1, de2, m.debotP1ui, m.debotP1uo)
				continue
			}

			// Check for collision with the second exit wire.
			if angle < p2a && nextAngle > p3a {
				m.dielBackFullySurrounded(p2ri, p2ro, p3ri, p3ro,
					angle, nextAngle, p3a, deback, de1, de2,
					m.debotP2ui, m.debotP2uo, m.debotP3ui, m.debotP3uo)
				continue
			}

			if angle < p2a && nextAngle > p2a {
				m.dielBackStraddleRight(p2ri, p2ro, angle, nextAngle, deback, de1, de2, m.debotP2ui, m.debotP2uo)
				continue
			}

			if angle > p2a && nextAngle < p3a {
				m.dielBackBetweenCenter(p2ri, p2ro, angle, nextAngle, deback, de1, de2)
				continue
			}

			if angle < p3a && nextAngle > p3a {
				m.dielBackStraddleLeft(p3ri, p3ro, angle, nextAngle, deback, de1, de2, m.debotP3ui, m.debotP3uo)
				continue
			}
		}

		// end cap
		m.dielTri(deback, de1, de2)
	}
}

func (m *arBifilarElectromagnet) dielBackStraddleRight(ri, ro, angle, nextAngle float64, deback, de1, de2, pui, puo *vec3.T) {
	x1i := float32(ri * math.Cos(angle))
	y1i := float32(ri * math.Sin(angle))
	x2i := float32(ri * math.Cos(nextAngle))
	y2i := float32(ri * math.Sin(nextAngle))
	x1o := float32(ro * math.Cos(angle))
	y1o := float32(ro * math.Sin(angle))
	x2o := float32(ro * math.Cos(nextAngle))
	y2o := float32(ro * math.Sin(nextAngle))

	de1i := &vec3.T{x1i, y1i, deback[2]}
	de2i := &vec3.T{x2i, y2i, deback[2]}
	m.dielTri(deback, de1i, pui)
	m.dielTri(deback, pui, de2i)

	de1o := &vec3.T{x1o, y1o, deback[2]}
	de2o := &vec3.T{x2o, y2o, deback[2]}
	m.dielTri(de1o, de1, puo)
	m.dielTri(puo, de1, de2)
	m.dielTri(puo, de2, de2o)

	m.dielQuad(de1i, pui, puo, de1o)
}

func (m *arBifilarElectromagnet) dielBackBetweenCenter(ri, ro, angle, nextAngle float64, deback, de1, de2 *vec3.T) {
	x1i := float32(ri * math.Cos(angle))
	y1i := float32(ri * math.Sin(angle))
	x2i := float32(ri * math.Cos(nextAngle))
	y2i := float32(ri * math.Sin(nextAngle))
	x1o := float32(ro * math.Cos(angle))
	y1o := float32(ro * math.Sin(angle))
	x2o := float32(ro * math.Cos(nextAngle))
	y2o := float32(ro * math.Sin(nextAngle))

	de1i := &vec3.T{x1i, y1i, deback[2]}
	de2i := &vec3.T{x2i, y2i, deback[2]}
	m.dielTri(deback, de1i, de2i)

	de1o := &vec3.T{x1o, y1o, deback[2]}
	de2o := &vec3.T{x2o, y2o, deback[2]}
	m.dielQuad(de1o, de2o, de2, de1)
}

func (m *arBifilarElectromagnet) dielBackStraddleLeft(ri, ro, angle, nextAngle float64, deback, de1, de2, pui, puo *vec3.T) {
	x1i := float32(ri * math.Cos(angle))
	y1i := float32(ri * math.Sin(angle))
	x2i := float32(ri * math.Cos(nextAngle))
	y2i := float32(ri * math.Sin(nextAngle))
	x1o := float32(ro * math.Cos(angle))
	y1o := float32(ro * math.Sin(angle))
	x2o := float32(ro * math.Cos(nextAngle))
	y2o := float32(ro * math.Sin(nextAngle))

	de1i := &vec3.T{x1i, y1i, deback[2]}
	de2i := &vec3.T{x2i, y2i, deback[2]}
	m.dielTri(deback, de1i, pui)
	m.dielTri(deback, pui, de2i)

	de1o := &vec3.T{x1o, y1o, deback[2]}
	de2o := &vec3.T{x2o, y2o, deback[2]}
	m.dielTri(de1o, de1, puo)
	m.dielTri(puo, de1, de2)
	m.dielTri(puo, de2, de2o)

	m.dielQuad(pui, de2i, de2o, puo)
}

func (m *arBifilarElectromagnet) dielBackFullySurrounded(rai, rao, rbi, rbo, angle, nextAngle, pba float64, deback, de1, de2, paui, pauo, pbui, pbuo *vec3.T) {
	const epsilon = 1e-4

	x1ai := float32(rai * math.Cos(angle))
	y1ai := float32(rai * math.Sin(angle))
	x1ao := float32(rao * math.Cos(angle))
	y1ao := float32(rao * math.Sin(angle))
	x2bi := float32(rbi * math.Cos(nextAngle))
	y2bi := float32(rbi * math.Sin(nextAngle))
	x2bo := float32(rbo * math.Cos(nextAngle))
	y2bo := float32(rbo * math.Sin(nextAngle))

	de1ai := &vec3.T{x1ai, y1ai, deback[2]}
	de2bi := &vec3.T{x2bi, y2bi, deback[2]}
	m.dielTri(deback, de1ai, paui)
	m.dielTri(deback, paui, pbui)
	if nextAngle-pba > epsilon {
		m.dielTri(deback, pbui, de2bi)
	}

	de1ao := &vec3.T{x1ao, y1ao, deback[2]}
	de2bo := &vec3.T{x2bo, y2bo, deback[2]}
	m.dielTri(de1ao, de1, pauo)
	m.dielTri(pauo, de1, de2)
	m.dielTri(pauo, de2, pbuo)
	if nextAngle-pba > epsilon {
		m.dielTri(pbuo, de2, de2bo)
	}

	m.dielQuad(de1ai, paui, pauo, de1ao)

	if nextAngle-pba > epsilon {
		m.dielQuad(pbui, de2bi, de2bo, pbuo)
	}
}
