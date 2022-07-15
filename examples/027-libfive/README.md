# 027-libfive

I recently came across this excellent CSG CAD system called
[libfive](https://libfive.com)
by Matthew Keeter and was amazed how well his ideas mesh (:smile:)
with IRMF.

Here is an adaptation of [libfive examples](https://libfive.com/examples)
in IRMF.

## Scheme (high-level)

This [libfive example](https://libfive.com/examples/#stdlib):

```scheme
(difference (sphere 1 [0 0 0])
  (sphere 0.6 [0 0 0])
  (cylinder-z 0.6 2 [0 0 -1])
  (reflect-xz (cylinder-z 0.6 2 [0 0 -1]))
  (reflect-yz (cylinder-z 0.6 2 [0 0 -1])))
```

can easily be translated to IRMF:

![libfive-1.png](libfive-1.png)

```glsl
void mainModel4(out vec4 m, in vec3 xyz) {
  m[0] = sphere(1.0, vec3(0), xyz);
  m[0] -= sphere(0.6, vec3(0), xyz);
  m[0] -= cylinder_z(0.6, 2.0, vec3(0,0,-1), xyz);
  m[0] -= cylinder_z(0.6, 2.0, vec3(0,0,-1), xyz.zyx);
  m[0] -= cylinder_z(0.6, 2.0, vec3(0,0,-1), xyz.xzy);
}
```

* Try loading [libfive-1.irmf](https://gmlewis.github.io/irmf-editor/?s=github.com/gmlewis/irmf-examples/blob/master/examples/027-libfive/libfive-1.irmf) now in the experimental IRMF editor!

* Use [irmf-slicer](https://github.com/gmlewis/irmf-slicer) to generate an STL or voxel approximation.

## Scheme (low-level)

The second [libfive example](https://libfive.com/examples/#scheme):

```scheme
(define-shape (cube x y z)
  (max (- x 1) (- -1 x)
       (- y 1) (- -1 y)
       (- z 1) (- -1 z)))

(remap-shape (cube x y z)
  (+ (* (cos z) x) (* (sin z) y))
  (- (* (cos z) y) (* (sin z) x))
  z)
```

could look like this in IRMF:

![libfive-2.png](libfive-2.png)

```glsl
float cube(in vec3 xyz) {
  if (any(greaterThan(abs(xyz), vec3(1)))) { return 0.0; }
  return 1.0;
}

void mainModel4(out vec4 m, in vec3 xyz) {
  float cz = cos(xyz.z);
  float sz = sin(xyz.z);
  vec3 twist = vec3(cz*xyz.x + sz*xyz.y, cz*xyz.y-sz*xyz.x, xyz.z);
  m[0] = cube(twist);
}
```

* Try loading [libfive-2.irmf](https://gmlewis.github.io/irmf-editor/?s=github.com/gmlewis/irmf-examples/blob/master/examples/027-libfive/libfive-2.irmf) now in the experimental IRMF editor!

* Use [irmf-slicer](https://github.com/gmlewis/irmf-slicer) to generate an STL or voxel approximation.


----------------------------------------------------------------------

# License

Copyright 2022 Glenn M. Lewis. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.