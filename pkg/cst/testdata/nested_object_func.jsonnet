local myObj = {
  key: 'val',
  objFunc(arg):: {
    funcKey: arg,
  },
};

local myArg = {
  argKey: 1,
};

[
  myObj.objFunc(myArg).funcKey.argKey,
]
