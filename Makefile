all:
	# go build --ldflags '-extldflags "-static"'
	# go build -ldflags='-s -w'   # release build
	go build -gcflags=all="-N -l"   # debug build
	# go build -ldflags "-linkmode external -extldflags -static" -gcflags=all="-N -l"   # debug build and static link
