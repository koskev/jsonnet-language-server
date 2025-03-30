local binaryObject =
  {
    one: 1,
    override: 'string',
  } +
  {
    two: 'two',
    override: {
      field: 5,
    },
  };


{
  a: binaryObject.one,
}
