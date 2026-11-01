module emby-stash-proxy

go 1.24.0

require (
	github.com/disintegration/imaging v1.6.3-0.20201218193011-d40f48ce0f09
	github.com/metatube-community/metatube-sdk-go v0.0.0
	golang.org/x/image v0.0.0-20200927104501-e162460cd6b5
)

require github.com/esimov/pigo v1.4.7-0.20240801095032-7465ed14de47 // indirect

replace github.com/metatube-community/metatube-sdk-go => ./third_party/metatube
