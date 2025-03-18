local data = { coolkey: 'hello' };

local localfunc(arg=data) = [
  arg.coolkey,
];

{
  a: localfunc(arg=data),
}
