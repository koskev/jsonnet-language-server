local pathOne = true;
local var = if pathOne then { firstCond: 'first' } else { secondCond: 'second' };
local func() = { first: var, second: 'hi' };

{
  res: func().first.firstCond,
  b: { a: 'hi' }.a,
}


//local var = { x: 'test' };  //if pathOne then { a: 'b' } else { b: 'second' };
//local func() = { a: var, b: 'hi' };
//
//
//func()
