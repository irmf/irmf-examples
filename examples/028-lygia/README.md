# 028-lygia

Here are examples of using the Lygia Shader Library with IRMF.

## lygia-01.irmf

```glsl
/*{
  irmf: "1.0",
  materials: ["copper"],
  max: [4,3,4],
  min: [-4,-3,-4],
  units: "mm",
}*/

#define FNC_SATURATE
#include "lygia/space/ratio.glsl"
#include "lygia/sdf.glsl"

vec4 raymarchMap(in vec3 pos){
  vec4 res=vec4(1.);

  res=opUnion(res,vec4(1.,1.,1.,sphereSDF(pos-vec3(0.,.60,0.),.5)));
  res=opUnion(res,vec4(0.,1.,1.,boxSDF(pos-vec3(2.,.5,0.),vec3(.4))));
  res=opUnion(res,vec4(.3,.3,1.,torusSDF(pos-vec3(0.,.5,2.),vec2(.4,.1))));
  res=opUnion(res,vec4(.3,.1,.3,capsuleSDF(pos,vec3(-2.3,.4,-.2),vec3(-1.6,.75,.2),.2)));
  res=opUnion(res,vec4(.5,.3,.4,triPrismSDF(pos-vec3(-2.,.50,-2.),vec2(.5,.1))));
  res=opUnion(res,vec4(.2,.2,.8,cylinderSDF(pos-vec3(2.,.50,-2.),vec2(.2,.4))));
  res=opUnion(res,vec4(.7,.5,.2,coneSDF(pos-vec3(0.,.75,-2.),vec3(.8,.6,.6))));
  res=opUnion(res,vec4(.4,.2,.9,hexPrismSDF(pos-vec3(-2.,.60,2.),vec2(.5,.1))));
  res=opUnion(res,vec4(.1,.3,.6,pyramidSDF(pos-vec3(2.,.10,2.),1.)));

  return res;
}

void mainModel4(out vec4 materials,in vec3 xyz){
  materials=smoothstep(vec4(1),vec4(.5),raymarchMap(xyz));
}
```

* Try loading [lygia-01.irmf](https://gmlewis.github.io/irmf-editor/?s=github.com/gmlewis/irmf-examples/blob/master/examples/028-lygia/lygia-01.irmf) now in the experimental IRMF editor!

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
