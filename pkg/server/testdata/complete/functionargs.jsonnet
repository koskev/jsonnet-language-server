local builder = import 'builderpattern.jsonnet';

local data = { coolkey: 'hello' };

local localfunc(arg=data) = [
  arg.coolkey,
];

local multiArguments(arg1, arg2, arg3) = {};

{
  a: localfunc(arg=data),
  b: multiArguments(1, 2, 3),
  c: builder.new('test'),
}
