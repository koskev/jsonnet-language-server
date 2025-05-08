#nvim ../pkg/server/testdata/complete/forobj.jsonnet
#$ delay 150

#$ send GO

#$ send b: forObj.one,\nc: forVar.
#$ sendarrow down 2
#$ send \r

#$ sendcontrol \

#$ send \x1b:q!\n
