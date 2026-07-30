package sites

import "github.com/ebash/barn/backend/internal/db"

func VolumeLinesFromSite(site db.Site) []string {
	return volumeLinesFromText(site.DockerVolumeMounts)
}

func NamedVolumeLinesFromSite(site db.Site) []string {
	return volumeLinesFromText(site.DockerNamedVolumes)
}
