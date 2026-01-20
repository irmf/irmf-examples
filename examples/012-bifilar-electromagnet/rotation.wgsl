// rotation.wgsl
// Copyright 2022 Glenn M. Lewis. All Rights Reserved.

fn rotAxis(axis: vec3f, a: f32) -> mat3x3f {
  let s = sin(a);
  let c = cos(a);
  let oc = 1.0 - c;
  let as_ = axis * s;
  let p = mat3x3f(axis.x * axis, axis.y * axis, axis.z * axis);
  let q = mat3x3f(c, as_.z, -as_.y, -as_.z, c, as_.x, as_.y, -as_.x, c);
  return p * oc + q;
}

fn rotZ(angle: f32) -> mat4x4f {
  let m = rotAxis(vec3f(0, 0, 1), angle);
  return mat4x4f(
    vec4f(m[0], 0.0),
    vec4f(m[1], 0.0),
    vec4f(m[2], 0.0),
    vec4f(0.0, 0.0, 0.0, 1.0)
  );
}
