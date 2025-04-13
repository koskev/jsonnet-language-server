local builder = import 'builderpattern.jsonnet';

[
  builder.new('test').withVal(1).withVal(1),
  builder.new('middle'),
  builder.new('last'),
]
