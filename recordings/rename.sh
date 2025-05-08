#nvim ../pkg/server/testdata/complete/function/multilintecall.jsonnet
#$ delay 150

#$ send 13G

#$ sendcontrol \
#$ send \x1b:lua vim.lsp.buf.rename()\r
#$ sendcontrol \

#$ wait 1000

#$ send new
#$ wait 1000
#$ send \r


#$ sendcontrol \
#$ send \x1b:q!\n
