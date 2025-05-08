#nvim ../pkg/server/testdata/complete/functionbody.jsonnet
#$ delay 150

#$ send GO

#$ send b: myFunc(exampleArg).field.argField,\n

#$ send c: myFunc({anotherField: "abc"}).field.

#$ wait 1000

#$ sendarrow down 1

#$ send \r

#$ wait 1000

#$ sendcontrol \

#$ send \x1b:q!\n
