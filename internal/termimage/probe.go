package termimage

import "github.com/charmbracelet/x/ansi"

const probeID = 31

// Terminal escape sequence for checking for kitty graphics support.
// Every terminal answers the query about device attributes. Terminals that support
// the graphics protocol will answer the first.
var probeQuery = ansi.KittyGraphics([]byte("AAAA"), "i=31", "s=1", "v=1", "a=q", "t=d", "f=24") +
	ansi.RequestPrimaryDeviceAttributes
