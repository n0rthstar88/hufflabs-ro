package main

import "io/fs"

// demoViews are sample screens for --demo and README screenshots.
func demoViews() map[string]View {
	return map[string]View{
		"checking":        {Phase: Checking, Title: "Checking for updates…", Fraction: -1},
		"scanning":        {Phase: Scanning, Title: "Checking game files…", Detail: "2140 of 6514 files", Fraction: 0.33},
		"downloading":     {Phase: Downloading, Title: "Updating HuffLabs RO", Detail: "Downloading 3 of 12 files — 41 MB of 120 MB", Fraction: 0.34},
		"offline":         {Phase: Offline, Title: "Can't reach the HuffLabs server", Detail: "Connect Tailscale, then press Retry.", Fraction: -1, CanRetry: true, CanPlay: true},
		"starting":        {Phase: Starting, Title: "Starting HuffLabs RO…", Fraction: 1},
		"start-missing":   {Phase: Failed, Title: "Couldn't start the game", Detail: startDetail(fs.ErrNotExist), Fraction: -1, CanRetry: true},
		"start-antivirus": {Phase: Failed, Title: "Couldn't start the game", Detail: "Windows Security or your antivirus blocked " + gameExe + ". Allow it under Protection history, then press Retry.", Fraction: -1, CanRetry: true},
		"start-policy":    {Phase: Failed, Title: "Couldn't start the game", Detail: "Windows blocked " + gameExe + " (Smart App Control or a security policy). Allow it, then press Retry.", Fraction: -1, CanRetry: true},
		"start-other":     {Phase: Failed, Title: "Couldn't start the game", Detail: "Windows said: The specified executable is not a valid application for this OS platform. Press Retry to check the files and try again.", Fraction: -1, CanRetry: true},
	}
}
