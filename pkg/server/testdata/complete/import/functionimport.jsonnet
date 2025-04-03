function(
  funcArg=(import 'function.libsonnet')({ number1: 1 }, { number2: 2 })
)
  {
    key1: funcArg.key1.number1,
  }
