// primitives.glsl
// Copyright 2022 Glenn M. Lewis. All Rights Reserved.

float box(vec3 start, vec3 end, float size, in vec3 xyz) {
  vec3 ll = min(start, end) - vec3(0.5 * size);
  vec3 ur = max(start, end) + vec3(0.5 * size);
  if (any(lessThan(xyz, ll))|| any(greaterThan(xyz, ur))) { return 0.0; }
  return 1.0;
}

mat3 rotAxis(vec3 axis, float a) {
  // This is from: http://www.neilmendoza.com/glsl-rotation-about-an-arbitrary-axis/
  float s = sin(a);
  float c = cos(a);
  float oc = 1.0 - c;
  vec3 as = axis * s;
  mat3 p = mat3(axis.x * axis, axis.y * axis, axis.z * axis);
  mat3 q = mat3(c, - as.z, as.y, as.z, c, - as.x, - as.y, as.x, c);
  return p * oc + q;
}

mat4 rotZ(float angle) {
  return mat4(rotAxis(vec3(0, 0, 1), angle));
}

float wire(vec3 start, vec3 end, float size, in vec3 xyz) {
  vec3 v = end - start;
  float angle = dot(v, vec3(1, 0, 0));
  xyz -= start;
  xyz = (vec4(xyz, 1) * rotZ(angle)).xyz;
  return box(vec3(0), vec3(length(v), 0, 0), size, xyz);
}

float cylinder(float radius, float height, in vec3 xyz) {
  // First, trivial reject on the two ends of the cylinder.
  if (xyz.z < 0.0 || xyz.z > height) { return 0.0; }
  
  // Then, constrain radius of the cylinder:
  float rxy = length(xyz.xy);
  if (rxy > radius) { return 0.0; }
  
  return 1.0;
}