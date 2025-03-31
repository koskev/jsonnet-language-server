local myObj = {
  key: 'val',
  objFunc(arg):: {
    funcKey: arg,
  },
};

[
  myObj.objFunc(5).funcKey,
]
