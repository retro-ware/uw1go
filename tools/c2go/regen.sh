cd /Users/felix/Projects/UltimaUnderworld && go build -o /tmp/c2go ./tools/c2go || exit 1
P=/private/tmp/claude-501/-Users-felix-Projects-UltimaUnderworld/c64b4361-ff4c-42a9-9e71-5f9e9e91252a/scratchpad/src/underworld-exhumed/uw1/src/port
for pair in 3d/camera:CAMERA 3d/expand:EXPAND 3d/instance:INSTANCE 3d/interp:INTERP 3d/noclip:NOCLIP 3d/pgcache_x:PGCACHE 3d/smooth:SMOOTH 3d/sphere:SPHERE 3d/tmapops:TMAPOPS gfx/grcore_x:GRCORE gfx/grdisp:GRDISP gfx/grentry:GRENTRY gfx/grjumps:GRJUMPS gfx/grlibf:GRLIBF gfx/grlibg:GRLIBG gfx/grlibh:GRLIBH gfx/grlibi:GRLIBI gfx/grlibl:GRLIBL gfx/grlibm:GRLIBM gfx/grlibn:GRLIBN gfx/grmisc:GRMISC gfx/polyfill:POLYFILL gfx/quadfit:QUADFIT gfx/scalebm:SCALEBM gfx/wallmap:WALLMAP gfx/vidmode:VIDMODE sys/c3dentry_x:C3DENTRY sys/imath:IMATH; do
 f=${pair%%:*}; n=${pair##*:}
 /tmp/c2go -name $n $P/$f.c > internal/r3d/m_${n:l}.go 2>/tmp/err_$n.txt || { echo "FAIL $n"; head -3 /tmp/err_$n.txt; }
done
gofmt -l internal/r3d >/dev/null
gofmt -w internal/r3d
