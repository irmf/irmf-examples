#!/bin/bash -ex
go run aprbfem.go -out aprbfem-11-1.stl -num_turns 1
go run aprbfem.go -out aprbfem-11-39.stl -num_turns 39
go run aprbfem.go -out aprbfem-11-3.stl -num_turns 3
go run aprbfem.go -out aprbfem-sapphire3d-850-500-11-19.stl -wire_gap 0.5 -num_turns 19 -inner_radius 3.9
go run aprbfem.go -out aprbfem-sapphire3d-850-500-11-39.stl -wire_gap 0.5 -num_turns 39 -inner_radius 3.9
go run aprbfem.go -out aprbfem-sapphire3d-850-500-11-9.stl -wire_gap 0.5 -num_turns 9 -inner_radius 3.9
