local localfunc(arg) = [
  arg,
];
local data = 'hello';

{
  a: localfunc(arg=data),
}
