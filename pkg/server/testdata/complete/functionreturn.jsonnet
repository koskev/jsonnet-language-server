local enablePath = true;

local conditional() =
  if enablePath then { one: 'pathone' } else { two: 'pathtwo' };

local conditionalStatic() = if true then { one: true } else { two: false };

local conditionalObj() = {
  one: 1,
  [if enablePath then 'two']: 2,
};

local conditionalArg(arg, pathOne) =
  if pathOne then { one: arg } else { two: arg };

local data = {
  b: 'hello',
};

local selfObj = {
  objFunc(): self,
  val: 'val',
};

[
  conditionalArg(data, true).one,
  conditionalArg(data, false).two,
  conditional().one,
  conditionalStatic().one,
  conditionalObj().two,
  selfObj.objFunc().val,
]
