//go:build windows

package main

import (
	"context"
	"image"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

func init() { runtime.LockOSThread() } // the window must live on the main thread

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	gdi32             = windows.NewLazySystemDLL("gdi32.dll")
	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	pRegisterClassEx  = user32.NewProc("RegisterClassExW")
	pCreateWindowEx   = user32.NewProc("CreateWindowExW")
	pDefWindowProc    = user32.NewProc("DefWindowProcW")
	pGetMessage       = user32.NewProc("GetMessageW")
	pTranslateMessage = user32.NewProc("TranslateMessage")
	pDispatchMessage  = user32.NewProc("DispatchMessageW")
	pBeginPaint       = user32.NewProc("BeginPaint")
	pEndPaint         = user32.NewProc("EndPaint")
	pInvalidateRect   = user32.NewProc("InvalidateRect")
	pSetTimer         = user32.NewProc("SetTimer")
	pPostQuitMessage  = user32.NewProc("PostQuitMessage")
	pPostMessage      = user32.NewProc("PostMessageW")
	pDestroyWindow    = user32.NewProc("DestroyWindow")
	pShowWindow       = user32.NewProc("ShowWindow")
	pLoadCursor       = user32.NewProc("LoadCursorW")
	pLoadIcon         = user32.NewProc("LoadIconW")
	pSetCursor        = user32.NewProc("SetCursor")
	pScreenToClient   = user32.NewProc("ScreenToClient")
	pGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	pTrackMouseEvent  = user32.NewProc("TrackMouseEvent")
	pSetDIBits        = gdi32.NewProc("SetDIBitsToDevice")
	pGetModuleHandle  = kernel32.NewProc("GetModuleHandleW")
)

const (
	wmDestroy     = 0x0002
	wmClose       = 0x0010
	wmPaint       = 0x000F
	wmSetCursor   = 0x0020
	wmNCHitTest   = 0x0084
	wmMouseMove   = 0x0200
	wmLButtonUp   = 0x0202
	wmMouseLeave  = 0x02A3
	wmTimer       = 0x0113
	wmApp         = 0x8000
	htClient      = 1
	htCaption     = 2
	wsPopup       = 0x80000000
	wsVisible     = 0x10000000
	wsMinimizeBox = 0x00020000
	wsExAppWindow = 0x00040000
	idcArrow      = 32512
	idcHand       = 32649
)

type wndclassex struct {
	Size, Style                uint32
	WndProc                    uintptr
	ClsExtra, WndExtra         int32
	Instance, Icon, Cursor, Bg windows.Handle
	MenuName, ClassName        *uint16
	IconSm                     windows.Handle
}

type winMsg struct {
	Hwnd    windows.HWND
	Message uint32
	W, L    uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type paintStruct struct {
	Hdc      windows.Handle
	Erase    int32
	Rc       [4]int32
	R, I     int32
	Reserved [32]byte
}

type bitmapInfoHeader struct {
	Size                   uint32
	Width, Height          int32
	Planes, BitCount       uint16
	Compression, SizeImage uint32
	XPels, YPels           int32
	ClrUsed, ClrImportant  uint32
}

type trackMouse struct {
	Size, Flags uint32
	Hwnd        windows.HWND
	Hover       uint32
}

type window struct {
	hwnd    windows.HWND
	mu      sync.Mutex
	view    View
	ui      uiState
	frame   *image.RGBA
	bgra    []byte
	choices chan Choice
	cancel  context.CancelFunc
	hand    uintptr
	arrow   uintptr
	tracked bool
}

var win *window // one window per process; wndProc finds it here

func lparamPoint(l uintptr) image.Point {
	return image.Pt(int(int16(l&0xffff)), int(int16((l>>16)&0xffff)))
}

func (w *window) choose(c Choice) {
	if c == Quit {
		w.cancel()
		pDestroyWindow.Call(uintptr(w.hwnd))
		return
	}
	select {
	case w.choices <- c:
	default: // the flow isn't waiting for a choice
	}
}

func wndProc(hwnd windows.HWND, m uint32, wp, lp uintptr) uintptr {
	w := win
	switch m {
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	case wmClose:
		w.choose(Quit)
		return 0
	case wmApp: // the flow finished
		pDestroyWindow.Call(uintptr(hwnd))
		return 0
	case wmTimer:
		w.mu.Lock()
		busy := w.view.Fraction < 0 && !w.view.CanRetry && !w.view.CanPlay
		if busy {
			w.ui.tick++
		}
		w.mu.Unlock()
		if busy {
			pInvalidateRect.Call(uintptr(hwnd), 0, 0)
		}
		return 0
	case wmNCHitTest:
		pt := lparamPoint(lp)
		p := struct{ X, Y int32 }{int32(pt.X), int32(pt.Y)}
		pScreenToClient.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&p)))
		w.mu.Lock()
		_, onButton := hit(buttons(w.view), image.Pt(int(p.X), int(p.Y)))
		w.mu.Unlock()
		if onButton {
			return htClient
		}
		return htCaption // drag the window anywhere else
	case wmSetCursor:
		w.mu.Lock()
		h := w.ui.hover
		w.mu.Unlock()
		if h != 0 {
			pSetCursor.Call(w.hand)
		} else {
			pSetCursor.Call(w.arrow)
		}
		return 1
	case wmMouseMove:
		if !w.tracked {
			tm := trackMouse{Size: uint32(unsafe.Sizeof(trackMouse{})), Flags: 2 /*TME_LEAVE*/, Hwnd: hwnd}
			pTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tm)))
			w.tracked = true
		}
		w.mu.Lock()
		c, _ := hit(buttons(w.view), lparamPoint(lp))
		changed := c != w.ui.hover
		w.ui.hover = c
		w.mu.Unlock()
		if changed {
			pInvalidateRect.Call(uintptr(hwnd), 0, 0)
		}
		return 0
	case wmMouseLeave:
		w.tracked = false
		w.mu.Lock()
		w.ui.hover = 0
		w.mu.Unlock()
		pInvalidateRect.Call(uintptr(hwnd), 0, 0)
		return 0
	case wmLButtonUp:
		w.mu.Lock()
		c, ok := hit(buttons(w.view), lparamPoint(lp))
		w.mu.Unlock()
		if ok {
			w.choose(c)
		}
		return 0
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := pBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		w.mu.Lock()
		render(w.frame, w.view, w.ui)
		w.mu.Unlock()
		pix := w.frame.Pix
		for i := 0; i < len(pix); i += 4 {
			w.bgra[i], w.bgra[i+1], w.bgra[i+2], w.bgra[i+3] = pix[i+2], pix[i+1], pix[i], 255
		}
		bi := bitmapInfoHeader{Size: 40, Width: W, Height: -H, Planes: 1, BitCount: 32}
		pSetDIBits.Call(hdc, 0, 0, W, H, 0, 0, 0, H, uintptr(unsafe.Pointer(&w.bgra[0])), uintptr(unsafe.Pointer(&bi)), 0)
		pEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		return 0
	}
	r, _, _ := pDefWindowProc.Call(uintptr(hwnd), uintptr(m), wp, lp)
	return r
}

// runUI shows the window, runs flow on another goroutine and returns its result
// when it finishes or the player closes the window.
func runUI(ctx context.Context, flow func(context.Context, func(View), <-chan Choice) Result) Result {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	w := &window{frame: image.NewRGBA(image.Rect(0, 0, W, H)), bgra: make([]byte, W*H*4), choices: make(chan Choice), cancel: cancel,
		view: View{Phase: Checking, Title: "Checking for updates…", Fraction: -1}}
	win = w
	w.arrow, _, _ = pLoadCursor.Call(0, idcArrow)
	w.hand, _, _ = pLoadCursor.Call(0, idcHand)
	inst, _, _ := pGetModuleHandle.Call(0)
	iconName, _ := windows.UTF16PtrFromString("APP")
	icon, _, _ := pLoadIcon.Call(inst, uintptr(unsafe.Pointer(iconName)))
	cls, _ := windows.UTF16PtrFromString("HuffLabsROLauncher")
	title, _ := windows.UTF16PtrFromString("HuffLabs RO")
	wc := wndclassex{Size: uint32(unsafe.Sizeof(wndclassex{})), WndProc: windows.NewCallback(wndProc), Instance: windows.Handle(inst),
		Icon: windows.Handle(icon), IconSm: windows.Handle(icon), Cursor: windows.Handle(w.arrow), ClassName: cls}
	pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	sw, _, _ := pGetSystemMetrics.Call(0)
	sh, _, _ := pGetSystemMetrics.Call(1)
	hwnd, _, _ := pCreateWindowEx.Call(wsExAppWindow, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)),
		wsPopup|wsVisible|wsMinimizeBox, (sw-W)/2, (sh-H)/2, W, H, 0, 0, inst, 0)
	w.hwnd = windows.HWND(hwnd)
	pShowWindow.Call(hwnd, 5)
	pSetTimer.Call(hwnd, 1, 33, 0)

	done := make(chan Result, 1)
	go func() {
		r := flow(ctx, func(v View) {
			w.mu.Lock()
			w.view = v
			w.mu.Unlock()
			pInvalidateRect.Call(hwnd, 0, 0)
		}, w.choices)
		done <- r
		pPostMessage.Call(hwnd, wmApp, 0, 0)
	}()
	var m winMsg
	for {
		r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if r == 0 || int32(r) == -1 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	cancel()
	return <-done
}
