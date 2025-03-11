local test2 = 5;
{
  functions:: {
    coolFunc(val='hi'): val,
    a(): test2,
  },
  x: 'hi',
  multiArgs(argOne, argTwo='two', argThree):: [
    argOne,
    argTwo,
    argThree,
  ],
}
