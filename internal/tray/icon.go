package tray

// iconData returns the tray icon as ICO bytes.
// This is a minimal 16x16 ICO with a speech-bubble shape in blue (#4A90D9).
func iconData() []byte {
	return generatedIcon
}

// generatedIcon is a 16x16 32-bit RGBA ICO file.
// Generated programmatically: blue speech bubble on transparent background.
var generatedIcon = generateIcon()

func generateIcon() []byte {
	const size = 16

	// Build 32-bit RGBA pixel data (BGRA order for ICO)
	pixels := make([]byte, size*size*4)

	// Speech bubble shape: filled rounded rect rows 2-10, tail at rows 11-12
	bubble := [16]uint16{
		// Each uint16 is a bitmask for which columns are filled (bit 0 = col 0)
		0x0000, // row 0
		0x0000, // row 1
		0x1FF8, // row 2:  cols 3-12
		0x3FFC, // row 3:  cols 2-13
		0x7FFE, // row 4:  cols 1-14
		0x7FFE, // row 5:  cols 1-14
		0x7FFE, // row 6:  cols 1-14
		0x7FFE, // row 7:  cols 1-14
		0x3FFC, // row 8:  cols 2-13
		0x1FF8, // row 9:  cols 3-12
		0x0FF0, // row 10: cols 4-11
		0x01E0, // row 11: cols 5-8  (tail)
		0x00C0, // row 12: cols 6-7  (tail tip)
		0x0000, // row 13
		0x0000, // row 14
		0x0000, // row 15
	}

	// Blue color: #4A90D9 → BGRA = D9, 90, 4A, FF
	bB, bG, bR, bA := byte(0xD9), byte(0x90), byte(0x4A), byte(0xFF)

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			off := (y*size + x) * 4
			if bubble[y]&(1<<uint(x)) != 0 {
				pixels[off+0] = bB
				pixels[off+1] = bG
				pixels[off+2] = bR
				pixels[off+3] = bA
			}
			// else: all zeros = transparent
		}
	}

	// ICO file format:
	// - 6 byte header
	// - 16 byte directory entry
	// - 40 byte BITMAPINFOHEADER
	// - pixel data (BGRA, bottom-up)
	// - AND mask (1bpp, bottom-up)

	andMaskRowBytes := ((size + 31) / 32) * 4 // 4 bytes per row for 16px
	andMaskSize := andMaskRowBytes * size
	bmpDataSize := size*size*4 + andMaskSize
	headerSize := 6 + 16
	bmpHeaderSize := 40
	totalSize := headerSize + bmpHeaderSize + bmpDataSize

	ico := make([]byte, totalSize)

	// ICO header
	ico[0] = 0 // reserved
	ico[1] = 0
	ico[2] = 1 // type: ICO
	ico[3] = 0
	ico[4] = 1 // count: 1 image
	ico[5] = 0

	// Directory entry (16 bytes at offset 6)
	ico[6] = byte(size)     // width
	ico[7] = byte(size)     // height
	ico[8] = 0              // colors (0 = no palette)
	ico[9] = 0              // reserved
	ico[10] = 1             // color planes
	ico[11] = 0
	ico[12] = 32            // bits per pixel
	ico[13] = 0
	// image size (little-endian uint32)
	imgSize := uint32(bmpHeaderSize + bmpDataSize)
	ico[14] = byte(imgSize)
	ico[15] = byte(imgSize >> 8)
	ico[16] = byte(imgSize >> 16)
	ico[17] = byte(imgSize >> 24)
	// offset to image data
	off := uint32(headerSize)
	ico[18] = byte(off)
	ico[19] = byte(off >> 8)
	ico[20] = byte(off >> 16)
	ico[21] = byte(off >> 24)

	// BITMAPINFOHEADER (40 bytes at offset 22)
	bmp := ico[22:]
	putLE32(bmp[0:], 40)           // header size
	putLE32(bmp[4:], uint32(size)) // width
	putLE32(bmp[8:], uint32(size*2)) // height (doubled for ICO: XOR + AND)
	bmp[12] = 1                    // planes
	bmp[13] = 0
	bmp[14] = 32                   // bpp
	bmp[15] = 0
	// compression, image size, ppm fields left as 0

	// Pixel data (bottom-up BGRA) at offset 62
	pixOff := 62
	for y := size - 1; y >= 0; y-- {
		for x := 0; x < size; x++ {
			srcOff := (y*size + x) * 4
			ico[pixOff+0] = pixels[srcOff+0] // B
			ico[pixOff+1] = pixels[srcOff+1] // G
			ico[pixOff+2] = pixels[srcOff+2] // R
			ico[pixOff+3] = pixels[srcOff+3] // A
			pixOff += 4
		}
	}

	// AND mask (bottom-up, 1bpp — 0 = opaque, 1 = transparent)
	for y := size - 1; y >= 0; y-- {
		var row [4]byte // 4 bytes for 16 pixels + padding
		for x := 0; x < size; x++ {
			srcOff := (y*size + x) * 4
			if pixels[srcOff+3] == 0 {
				// transparent: set bit
				row[x/8] |= 1 << uint(7-x%8)
			}
		}
		for i := 0; i < andMaskRowBytes; i++ {
			ico[pixOff] = row[i]
			pixOff++
		}
	}

	return ico
}

func putLE32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
