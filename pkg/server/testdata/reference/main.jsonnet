local l = import 'lib.libsonnet';

[
  l.functions.coolFunc(),
  l
  .
    functions
  .
    a(),
  l.x,
  l.nestedOne.nestedTwo.nestedThree.nestedFour.test4,
]
