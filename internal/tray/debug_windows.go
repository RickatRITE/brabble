// +build windows

package tray

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	registerClassEx    = user32.NewProc("RegisterClassExW")
	createWindowEx     = user32.NewProc("CreateWindowExW")
	defWindowProc      = user32.NewProc("DefWindowProcW")
	getMessage         = user32.NewProc("GetMessageW")
	translateMessage   = user32.NewProc("TranslateMessage")
	dispatchMessage    = user32.NewProc("DispatchMessageW")
	postQuitMessage    = user32.NewProc("PostQuitMessage")
	showWindow         = user32.NewProc("ShowWindow")
	updateWindow       = user32.NewProc("UpdateWindow")
	setTimer           = user32.NewProc("SetTimer")
	invalidateRect     = user32.NewProc("InvalidateRect")
	beginPaint         = user32.NewProc("BeginPaint")
	endPaint           = user32.NewProc("EndPaint")
	getClientRect      = user32.NewProc("GetClientRect")
	fillRect           = user32.NewProc("FillRect")
	setTextColor       = gdi32.NewProc("SetTextColor")
	setBkMode          = gdi32.NewProc("SetBkMode")
	createFontIndirect = gdi32.NewProc("CreateFontIndirectW")
	selectObject       = gdi32.NewProc("SelectObject")
	deleteObject       = gdi32.NewProc("DeleteObject")
	drawTextW          = user32.NewProc("DrawTextW")
	createSolidBrush   = gdi32.NewProc("CreateSolidBrush")
	setWindowPos       = user32.NewProc("SetWindowPos")
	getModuleHandle    = kernel32.NewProc("GetModuleHandleW")
)

const (
	wsOverlappedWindow = 0x00CF0000
	wsExTopmost        = 0x00000008
	swShow             = 5
	wmDestroy          = 0x0002
	wmPaint            = 0x000F
	wmTimer            = 0x0113
	csHRedraw          = 0x0002
	csVRedraw          = 0x0001
	dtLeft             = 0x0000
	dtWordBreak        = 0x0010
	transparent        = 1
	hwndTopmost        = ^uintptr(0) // -1
	swpNoSize          = 0x0001
	swpNoMove          = 0x0002
	swpNoActivate      = 0x0010
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

type point struct {
	x, y int32
}

type msg struct {
	hwnd    syscall.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type rect struct {
	left, top, right, bottom int32
}

type paintStruct struct {
	hdc         syscall.Handle
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type logFont struct {
	lfHeight         int32
	lfWidth          int32
	lfEscapement     int32
	lfOrientation    int32
	lfWeight         int32
	lfItalic         byte
	lfUnderline      byte
	lfStrikeOut      byte
	lfCharSet        byte
	lfOutPrecision   byte
	lfClipPrecision  byte
	lfQuality        byte
	lfPitchAndFamily byte
	lfFaceName       [32]uint16
}

type debugState struct {
	DB       float64 `json:"db"`
	VAD      bool    `json:"vad"`
	Speech   bool    `json:"speech"`
	ChunkSec int     `json:"chunk_sec"`
	TS       string  `json:"ts"`
}

var (
	currentState   debugState
	lastHeard      string
	heardLog       []string // rolling log of all transcriptions
	heardLogMax    = 10
	debugHwnd      syscall.Handle
)

func rgb(r, g, b byte) uintptr {
	return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16
}

func utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

// RunDebugMonitor creates a native Win32 always-on-top debug window.
func RunDebugMonitor() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInst, _, _ := getModuleHandle.Call(0)
	className := utf16Ptr("BrabbleDebug")

	bgBrush, _, _ := createSolidBrush.Call(rgb(30, 30, 30))

	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		style:         csHRedraw | csVRedraw,
		lpfnWndProc:   syscall.NewCallback(debugWndProc),
		hInstance:     syscall.Handle(hInst),
		hbrBackground: syscall.Handle(bgBrush),
		lpszClassName: className,
	}
	registerClassEx.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ := createWindowEx.Call(
		wsExTopmost,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16Ptr("Brabble Debug"))),
		wsOverlappedWindow,
		20, 20, 450, 420, // x, y, w, h
		0, 0, hInst, 0,
	)
	debugHwnd = syscall.Handle(hwnd)
	showWindow.Call(hwnd, swShow)
	updateWindow.Call(hwnd)

	// Keep topmost
	setWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoSize|swpNoMove|swpNoActivate)

	// 100ms timer for updates
	setTimer.Call(hwnd, 1, 100, 0)

	var m msg
	for {
		ret, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if ret == 0 {
			break
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func debugWndProc(hwnd syscall.Handle, umsg uint32, wParam, lParam uintptr) uintptr {
	switch umsg {
	case wmTimer:
		readDebugState()
		invalidateRect.Call(uintptr(hwnd), 0, 1)
		return 0
	case wmPaint:
		paintDebug(hwnd)
		return 0
	case wmDestroy:
		postQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := defWindowProc.Call(uintptr(hwnd), uintptr(umsg), wParam, lParam)
	return ret
}

func readDebugState() {
	debugFile := filepath.Join(os.TempDir(), "brabble-debug.json")
	data, err := os.ReadFile(debugFile)
	if err == nil {
		var s debugState
		if json.Unmarshal(data, &s) == nil {
			currentState = s
		}
	}

	// Read all "heard" lines from log and build rolling transcript
	logFile := filepath.Join(os.Getenv("LOCALAPPDATA"), "brabble", "brabble.log")
	logData, err := os.ReadFile(logFile)
	if err != nil {
		return
	}
	var allHeard []string
	lines := strings.Split(string(logData), "\n")
	for _, line := range lines {
		// Log format: msg="heard: \"actual text\""
		marker := `heard: \"`
		if idx := strings.Index(line, marker); idx >= 0 {
			start := idx + len(marker)
			// Find closing \"" at end of msg
			end := strings.Index(line[start:], `\"`)
			if end > 0 {
				text := line[start : start+end]
				// Extract timestamp (HH:MM:SS) from log line
				ts := ""
				if tIdx := strings.Index(line, "T"); tIdx >= 0 {
					tsEnd := tIdx + 9 // T + HH:MM:SS
					if tsEnd <= len(line) {
						ts = line[tIdx+1 : tsEnd]
					}
				}
				entry := text
				if ts != "" {
					entry = ts + "  " + entry
				}
				allHeard = append(allHeard, entry)
			}
		}
	}
	// Keep last N entries
	if len(allHeard) > heardLogMax {
		allHeard = allHeard[len(allHeard)-heardLogMax:]
	}
	heardLog = allHeard
	if len(allHeard) > 0 {
		lastHeard = allHeard[len(allHeard)-1]
	}
}

func paintDebug(hwnd syscall.Handle) {
	var ps paintStruct
	hdc, _, _ := beginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

	var rc rect
	getClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))

	// Background
	bgBrush, _, _ := createSolidBrush.Call(rgb(30, 30, 30))
	fillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), bgBrush)
	deleteObject.Call(bgBrush)

	setBkMode.Call(hdc, transparent)

	// Create font
	lf := logFont{lfHeight: -16, lfWeight: 400, lfCharSet: 1, lfQuality: 5}
	copy(lf.lfFaceName[:], syscall.StringToUTF16("Consolas"))
	font, _, _ := createFontIndirect.Call(uintptr(unsafe.Pointer(&lf)))
	oldFont, _, _ := selectObject.Call(hdc, font)

	y := int32(10)
	margin := int32(12)

	// Level + meter
	db := currentState.DB
	levelText := fmt.Sprintf("Level: %.1f dB", db)
	setTextColor.Call(hdc, rgb(0, 255, 0))
	drawLine(hdc, levelText, margin, y, rc.right-margin)
	y += 22

	// Draw meter bar
	meterWidth := rc.right - margin*2
	pct := (db + 60) / 60
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	barW := int32(float64(meterWidth) * pct)

	// Meter background
	meterBg := rect{left: margin, top: y, right: margin + meterWidth, bottom: y + 14}
	darkBrush, _, _ := createSolidBrush.Call(rgb(60, 60, 60))
	fillRect.Call(hdc, uintptr(unsafe.Pointer(&meterBg)), darkBrush)
	deleteObject.Call(darkBrush)

	// Meter fill
	if barW > 0 {
		var barColor uintptr
		if currentState.VAD {
			barColor = rgb(0, 220, 0)
		} else {
			barColor = rgb(100, 100, 100)
		}
		meterFill := rect{left: margin, top: y, right: margin + barW, bottom: y + 14}
		fillBrush, _, _ := createSolidBrush.Call(barColor)
		fillRect.Call(hdc, uintptr(unsafe.Pointer(&meterFill)), fillBrush)
		deleteObject.Call(fillBrush)
	}
	y += 22

	// VAD
	if currentState.VAD {
		setTextColor.Call(hdc, rgb(0, 255, 0))
		drawLine(hdc, "VAD: ACTIVE", margin, y, rc.right-margin)
	} else {
		setTextColor.Call(hdc, rgb(130, 130, 130))
		drawLine(hdc, "VAD: silent", margin, y, rc.right-margin)
	}
	y += 20

	// Speech
	if currentState.Speech {
		setTextColor.Call(hdc, rgb(255, 180, 0))
		drawLine(hdc, fmt.Sprintf("Speech: RECORDING (%ds)", currentState.ChunkSec), margin, y, rc.right-margin)
	} else {
		setTextColor.Call(hdc, rgb(130, 130, 130))
		drawLine(hdc, "Speech: idle", margin, y, rc.right-margin)
	}
	y += 20

	// Threshold
	setTextColor.Call(hdc, rgb(100, 100, 100))
	drawLine(hdc, "Energy threshold: -35 dB", margin, y, rc.right-margin)
	y += 24

	// Transcript header
	setTextColor.Call(hdc, rgb(180, 180, 180))
	drawLine(hdc, "── Transcript ──", margin, y, rc.right-margin)
	y += 18

	// Rolling transcript log
	if len(heardLog) == 0 {
		setTextColor.Call(hdc, rgb(100, 100, 100))
		drawLine(hdc, "(nothing heard yet)", margin, y, rc.right-margin)
	} else {
		for _, entry := range heardLog {
			setTextColor.Call(hdc, rgb(255, 200, 0))
			drawLine(hdc, entry, margin, y, rc.right-margin)
			y += 16
			if y > rc.bottom-24 {
				break
			}
		}
	}

	// Timestamp in bottom-right
	if currentState.TS != "" {
		setTextColor.Call(hdc, rgb(80, 80, 80))
		tsRect := rect{left: rc.right - 100, top: rc.bottom - 20, right: rc.right - margin, bottom: rc.bottom - 4}
		ts16, _ := syscall.UTF16FromString(currentState.TS)
		drawTextW.Call(hdc, uintptr(unsafe.Pointer(&ts16[0])), uintptr(len(ts16)-1),
			uintptr(unsafe.Pointer(&tsRect)), dtLeft)
	}

	selectObject.Call(hdc, oldFont)
	deleteObject.Call(font)
	endPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
}

func drawLine(hdc uintptr, text string, x, y, right int32) {
	r := rect{left: x, top: y, right: right, bottom: y + 20}
	t, _ := syscall.UTF16FromString(text)
	drawTextW.Call(hdc, uintptr(unsafe.Pointer(&t[0])), uintptr(len(t)-1),
		uintptr(unsafe.Pointer(&r)), dtLeft)
}

// readLastHeard returns the most recent transcription from the log (called externally).
func readLastHeard() string {
	return lastHeard
}

// init sets defaults
func init() {
	currentState = debugState{DB: -60}
	lastHeard = ""
}
