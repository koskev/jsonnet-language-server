local forObj =
  {
    [x]: x
    for x in ['one', 'two']
  };

{
  a: forObj.one,
}
