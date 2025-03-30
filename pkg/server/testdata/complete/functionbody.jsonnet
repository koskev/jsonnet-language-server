local myFunc(arg) = { field: arg };

local exampleArg = {
  argField: 5,
};

{
  a: myFunc(exampleArg).field,
}
