package headers

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	http "github.com/bogdanfinn/fhttp"
)

type viewport struct {
	Width      int
	Height     int
	PixelRatio float64
}

type clientHints struct {
	ConnectionType string
	EffectiveType  string
	Downlink       float64
	RTT            int
	SaveData       string
}

type Profile struct {
	ua           string
	secCHUA      string
	viewport     viewport
	hints        clientHints
	acceptIdx    int
	langIdx      int
	encIdx       int
	cacheIdx     int
	memIdx       int
	viewportProb []float64
	hintsProb    []float64
	memProb      float64
}

var (
	acceptOpts = []string{
		"application/json",
		"application/json, text/plain, */*",
		"application/json, text/javascript, */*; q=0.01",
	}
	encOpts = []string{
		"gzip, deflate, br",
		"gzip, deflate, br, zstd",
		"br, gzip, deflate",
	}
	langOpts = []string{
		"en-US,en;q=0.9",
		"en-US,en;q=0.8",
		"en-GB,en;q=0.9,en-US;q=0.8",
		"en-CA,en;q=0.9,en-US;q=0.8",
		"en,en-US;q=0.9",
		"en-US,en;q=0.9,es;q=0.8",
		"en-US",
	}
	cacheOpts = []string{
		"max-age=0",
		"no-cache",
		"max-age=0, private, must-revalidate",
		"",
	}
	memOpts = []string{"2", "4", "8"}

	headerOrder = []string{
		"Accept",
		"Accept-Language",
		"Accept-Encoding",
		"User-Agent",
		"Sec-CH-UA",
		"Sec-CH-UA-Mobile",
		"Sec-CH-UA-Platform",
		"Sec-Fetch-Site",
		"Sec-Fetch-Mode",
		"Sec-Fetch-Dest",
		"Sec-CH-Viewport-Width",
		"Sec-CH-Viewport-Height",
		"Sec-CH-DPR",
		"Sec-CH-UA-Netinfo",
		"Sec-CH-UA-Downlink",
		"Sec-CH-UA-RTT",
		"Save-Data",
		"Device-Memory",
		"Connection",
		"Cache-Control",
		"Origin",
		"Referer",
		"Priority",
	}
)

var profilePool = sync.Pool{
	New: func() interface{} {
		return generateProfile()
	},
}

func randomBuildToken(n int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func generateRandomViewport() viewport {
	w := rand.Intn(80) + 360
	h := rand.Intn(276) + 640
	dprChoices := []float64{2, 2.5, 3, 3.5}
	return viewport{Width: w, Height: h, PixelRatio: dprChoices[rand.Intn(len(dprChoices))]}
}

func generateClientHints() clientHints {
	connOpts := []string{"keep-alive", "close", ""}
	effOpts := []string{"slow-2g", "2g", "3g", "4g", ""}
	saveOpts := []string{"on", ""}
	return clientHints{
		ConnectionType: connOpts[rand.Intn(len(connOpts))],
		EffectiveType:  effOpts[rand.Intn(len(effOpts))],
		Downlink:       rand.Float64()*9.9 + 0.1,
		RTT:            rand.Intn(251) + 50,
		SaveData:       saveOpts[rand.Intn(len(saveOpts))],
	}
}

func generateRandomUA() string {
	androidVer := rand.Intn(10) + 8
	architectures := []string{"arm64-v8a", "armeabi-v7a", "x86", "x86_64"}
	arch := architectures[rand.Intn(len(architectures))]

	switch rand.Intn(5) {
	case 0: // Android Chrome
		maj := rand.Intn(11) + 130
		min := rand.Intn(10)
		return fmt.Sprintf(
			"Mozilla/5.0 (Linux; Android %d; AndroidDeviceModel%02d; %s Build/%s) "+
				"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.%d.0 Mobile Safari/537.36 XTR/%s",
			androidVer, rand.Intn(80)+1, arch, randomBuildToken(5),
			maj, min, randomBuildToken(5),
		)
	case 1: // iOS Safari
		maj := rand.Intn(8) + 12
		min := rand.Intn(10)
		device := fmt.Sprintf("iPhone%02d", rand.Intn(80)+6)
		return fmt.Sprintf(
			"Mozilla/5.0 (%s; CPU %s OS %d_%d like Mac OS X) "+
				"AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%d.%d Mobile/%s Safari/604.1 XTR/%s",
			device, device, maj, min, maj, rand.Intn(2), randomBuildToken(5), randomBuildToken(5),
		)
	case 2: // Samsung Internet
		sbMaj := rand.Intn(7) + 20
		sbMin := rand.Intn(10)
		return fmt.Sprintf(
			"Mozilla/5.0 (Linux; Android %d; SAMSUNG SM-G%03d; %s Build/%s) "+
				"AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/%d.%d "+
				"Chrome/%d.0.%d.0 Mobile Safari/537.36 XTR/%s",
			androidVer, rand.Intn(80)+900, arch, randomBuildToken(5),
			sbMaj, sbMin, rand.Intn(11)+130, rand.Intn(10), randomBuildToken(5),
		)
	case 3: // Android Firefox
		rvMaj := rand.Intn(16) + 115
		rvMin := rand.Intn(10)
		return fmt.Sprintf(
			"Mozilla/5.0 (Android %d; %s; Mobile; rv:%d.%d) Gecko/%d.%d Firefox/%d.%d Build/%s XTR/%s",
			androidVer, arch, rvMaj, rvMin, rvMaj, rvMin, rvMaj, rvMin, randomBuildToken(5), randomBuildToken(5),
		)
	default: // Edge Mobile
		edgeMaj := rand.Intn(11) + 120
		edgeMin := rand.Intn(10)
		return fmt.Sprintf(
			"Mozilla/5.0 (Linux; Android %d; EdgeDeviceModel%02d; %s Build/%s) "+
				"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.%d.0 Mobile Safari/537.36 EdgA/%d.%d XTR/%s",
			androidVer, rand.Intn(80)+1, arch, randomBuildToken(5),
			rand.Intn(11)+130, rand.Intn(10), edgeMaj, edgeMin, randomBuildToken(5),
		)
	}
}

func generateSecCHUA(ua string) string {
	const fallback = "136.0.0.0"
	ver := fallback
	if idx := strings.Index(ua, "Chrome/"); idx != -1 {
		rest := ua[idx+7:]
		if j := strings.Index(rest, " "); j != -1 {
			ver = rest[:j]
		} else {
			ver = rest
		}
	}
	return fmt.Sprintf(
		`"Not:A-Brand";v="24", "Chromium";v="%s", "Google Chrome";v="%s"`,
		ver, ver,
	)
}

func generateProfile() Profile {
	ua := generateRandomUA()

	viewportProbs := make([]float64, 3)
	for i := range viewportProbs {
		viewportProbs[i] = rand.Float64()
	}

	hintsProbs := make([]float64, 5)
	for i := range hintsProbs {
		hintsProbs[i] = rand.Float64()
	}

	return Profile{
		ua:           ua,
		secCHUA:      generateSecCHUA(ua),
		viewport:     generateRandomViewport(),
		hints:        generateClientHints(),
		acceptIdx:    rand.Intn(len(acceptOpts)),
		langIdx:      rand.Intn(len(langOpts)),
		encIdx:       rand.Intn(len(encOpts)),
		cacheIdx:     rand.Intn(len(cacheOpts)),
		memIdx:       rand.Intn(len(memOpts)),
		viewportProb: viewportProbs,
		hintsProb:    hintsProbs,
		memProb:      rand.Float64(),
	}
}

func BuildHeaders(tcin string) http.Header {
	profile := profilePool.Get().(Profile)
	defer profilePool.Put(profile)

	h := http.Header{}
	h.Set("Accept", acceptOpts[profile.acceptIdx])
	h.Set("Accept-Language", langOpts[profile.langIdx])
	h.Set("Accept-Encoding", encOpts[profile.encIdx])
	h.Set("User-Agent", profile.ua)
	h.Set("Sec-CH-UA", profile.secCHUA)
	h.Set("Sec-CH-UA-Mobile", func() string {
		if strings.Contains(profile.ua, "Mobile") {
			return "?1"
		}
		return "?0"
	}())
	h.Set("Sec-CH-UA-Platform", `"`+func() string {
		if strings.Contains(profile.ua, "Android") {
			return "Android"
		}
		return "macOS"
	}()+`"`)
	h.Set("Origin", "https://www.target.com")
	h.Set("Referer", fmt.Sprintf(
		"https://www.target.com/p/2023-panini-select-baseball-trading-card-blaster-box/-/A-%s",
		tcin,
	))
	h.Set("Sec-Fetch-Site", "same-site")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("Sec-Fetch-Dest", "empty")
	h.Set("Priority", "u=1,i")

	if cc := cacheOpts[profile.cacheIdx]; cc != "" {
		h.Set("Cache-Control", cc)
	}

	if profile.viewportProb[0] < 0.7 {
		h.Set("Sec-CH-Viewport-Width", strconv.Itoa(profile.viewport.Width))
	}
	if profile.viewportProb[1] < 0.7 {
		h.Set("Sec-CH-Viewport-Height", strconv.Itoa(profile.viewport.Height))
	}
	if profile.viewportProb[2] < 0.7 {
		h.Set("Sec-CH-DPR", fmt.Sprintf("%.1f", profile.viewport.PixelRatio))
	}

	if profile.hints.ConnectionType != "" && profile.hintsProb[0] < 0.6 {
		h.Set("Connection", profile.hints.ConnectionType)
	}
	if profile.hints.EffectiveType != "" && profile.hintsProb[1] < 0.5 {
		h.Set("Sec-CH-UA-Netinfo", profile.hints.EffectiveType)
	}
	if profile.hintsProb[2] < 0.4 {
		h.Set("Sec-CH-UA-Downlink", fmt.Sprintf("%.1f", profile.hints.Downlink))
	}
	if profile.hintsProb[3] < 0.3 {
		h.Set("Sec-CH-UA-RTT", strconv.Itoa(profile.hints.RTT))
	}
	if profile.hints.SaveData == "on" {
		h.Set("Save-Data", "on")
	}

	if profile.memProb < 0.3 {
		h.Set("Device-Memory", memOpts[profile.memIdx])
	}

	h[http.HeaderOrderKey] = headerOrder

	return h
}

func InitProfilePool(count int) {
	profiles := make([]interface{}, count)
	for i := 0; i < count; i++ {
		profiles[i] = generateProfile()
	}
	for _, profile := range profiles {
		profilePool.Put(profile)
	}
}

func ResetProfilePool() {
	profilePool = sync.Pool{New: func() interface{} { return generateProfile() }}
}