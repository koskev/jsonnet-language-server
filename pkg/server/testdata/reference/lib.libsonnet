local test2 = 5;
{
  functions:: {
    coolFunc(val='hi'): val,
    a(): test2,
  },
  x: 'ih',
  multiArgs(argOne, argTwo='two', argThree):: [
    argOne,
    argTwo,
    argThree,
  ],
  nestedOne:: {
    test1: 'nestedTest',
    nestedTwo:: {
      test2: 'nestedTest',
      nestedThree:: {
        test3: 'nestedTest',
        nestedFour:: {
          test4: 'nestedTest',
        },
      },
    },
  },
}
