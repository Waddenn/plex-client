package appinfo

import "runtime"

var Version = "dev"

type Info struct {
	Product   string
	Version   string
	Platform  string
	Device    string
	UserAgent string
}

func Default() Info {
	info := Info{
		Product:  "Plex Client",
		Version:  Version,
		Platform: runtime.GOOS,
		Device:   "Plex Client",
	}
	info.UserAgent = info.Product + "/" + info.Version
	return info
}
