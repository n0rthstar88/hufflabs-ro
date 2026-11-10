package main

// demoViews are sample screens for --demo and README screenshots.
func demoViews() map[string]View {
	return map[string]View{
		"checking":    {Phase: Checking, Title: "Checking for updates…", Fraction: -1},
		"scanning":    {Phase: Scanning, Title: "Checking game files…", Detail: "2140 of 6514 files", Fraction: 0.33},
		"downloading": {Phase: Downloading, Title: "Updating HuffLabs RO", Detail: "Downloading 3 of 12 files — 41 MB of 120 MB", Fraction: 0.34},
		"offline":     {Phase: Offline, Title: "Can't reach the HuffLabs server", Detail: "Connect Tailscale, then press Retry.", Fraction: -1, CanRetry: true, CanPlay: true},
		"starting":    {Phase: Starting, Title: "Starting HuffLabs RO…", Fraction: 1},
	}
}
