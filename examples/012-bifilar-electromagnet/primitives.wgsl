// primitives.wgsl
// Copyright 2022 Glenn M. Lewis. All Rights Reserved.

fn box_prim(start: vec3f, end: vec3f, size: f32, xyz: vec3f) -> f32 {
  let ll = min(start, end) - vec3f(0.5 * size);
  let ur = max(start, end) + vec3f(0.5 * size);
  if (any(xyz < ll) || any(xyz > ur)) { return 0.0; }
  return 1.0;
}

// Note: wire requires rotZ which is in rotation.wgsl
// If including both, ensure no naming conflicts.
// Since WGSL doesn't have a standard #include that works everywhere,
// it might be better to just inline these in the IRMF files.
// But for now, let's keep them separate if the user wants them.

fn cylinder(radius: f32, height: f32, xyz: vec3f) -> f32 {
  // First, trivial reject on the two ends of the cylinder.
  if (xyz.z < 0.0 || xyz.z > height) { return 0.0; }
  
  // Then, constrain radius of the cylinder:
  let rxy = length(xyz.xy);
  if (rxy > radius) { return 0.0; }
  
  return 1.0;
}
