local myArray = ['one', 'two'];

local forVar =
  {
    [x]: x
    for x in myArray
  };

local forObj =
  {
    [x]: x
    for x in ['one', 'two']
  };

{
  a: forObj.one,
}
