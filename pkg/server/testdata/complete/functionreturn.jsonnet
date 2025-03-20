local enablePath = true;

local conditional() =
  if enablePath then { one: 'pathone' } else { two: 'pathtwo' };

local conditionalStatic() = if true then { one: true } else { two: false };

local conditionalFunc() = {
  one: 1,
  [if enablePath then 'two']: 2,
};

local conditionalObj =
  {
    one: 1,
    [if enablePath then 'two']: 2,
  };

local forObj =
  {
    [x]: x
    for x in ['one', 'two', 'three']
  };

local compoundObject =
  {
    one: 1,
  } +
  {
    two: 2,
  };

local conditionalArgs(arg, pathOne) =
  if pathOne then { one: arg } else { two: arg };

local conditionalArg(pathOne) =
  if pathOne then { one: 'one' } else { two: 'two' };

local data = {
  b: 'hello',
};

local selfObj = {
  objFunc():: self + { obj: 'object' },
  val:: 'val',
};

local impossibleComplete = {
  [if enablePath then 'two']:: {
    myFunc():: {
      val: 'val',
    },
  },
};

[
  conditionalArgs(data, true).one,
  conditionalArgs(data, false).two,
  conditional(),
  conditionalStatic().one,
  conditionalObj.two,
  selfObj.objFunc().val,
  conditionalObj,
  compoundObject,
  forObj.three,
  conditionalArg(false).two,
  selfObj.objFunc(),
  selfObj,
]
