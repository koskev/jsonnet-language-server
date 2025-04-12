{
  new(name):: {
    assert std.isString(name),
    local data = self,
    _name: name,
    _vals: [],


    withName(name):: self {
      assert std.isString(name),
      _name: name,
    },

    withVal(arg)::
      local val = arg;
      assert std.isNumber(val);
      self {
        assert std.isNumber(val),
        _val: data._vals + [val],
      },
  },

  test: self.new('mybuilder'),
}
