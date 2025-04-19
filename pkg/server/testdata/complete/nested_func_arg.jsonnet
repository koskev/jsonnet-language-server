{

  func1(arg1)::
    assert std.isObject(arg1);
    self,

  func2(arg2):: self.func1({
    key: arg2,
  }),

  test:: self.func2({}),
}
