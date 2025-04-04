local data = {
  nested1: {
    nested2Func():: {
      nested3: {
        nested4Func(arg): {
          nested5: arg,
        },
      },
    },

  },
};

local data2 = {
  nested2data: {
    nested2data2: 'val',
  },

};

local localfunc(arg=data) = {
  funcBody: arg,
};

{
  a: localfunc(arg=data).funcBody.nested1.nested2Func().nested3.nested4Func(data2).nested5.nested2data.nested2data2,
}
