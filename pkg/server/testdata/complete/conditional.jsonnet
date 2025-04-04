local localcondition = 50 == 50;
local myCondArg(condition) = { base: 5 } + if condition then { trueField: true } else { falseField: false };
local myCondLocal() = { base: 5 } + if localcondition then { trueField: true } else { falseField: false };

{
  a: myCondLocal(),
}
