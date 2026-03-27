package control

import (
	"os"
	"testing"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

func TestRemoveWakeWordLocal(t *testing.T) {
	cases := []struct {
		text   string
		word   string
		expect string
	}{
		{"clawd make it so", "clawd", "make it so"},
		{"clawd, launch torpedo", "clawd", "launch torpedo"},
		{"no wake here", "clawd", "no wake here"},
		{"hey computer, refactor this", "hey computer", "refactor this"},
		{"Hey Computer do something", "hey computer", "do something"},
		{"Claude! help me", "claude", "help me"},
	}
	for _, c := range cases {
		if got := removeWakeWordLocal(c.text, c.word); got != c.expect {
			t.Fatalf("removeWakeWordLocal(%q)=%q want %q", c.text, got, c.expect)
		}
	}
}

func TestResampleLinearLength(t *testing.T) {
	in := []float32{0, 1, 2, 3}
	out := resampleLinear(in, 16000, 8000)
	if len(out) != 2 {
		t.Fatalf("downsample length got %d", len(out))
	}
	out = resampleLinear(in, 8000, 16000)
	if len(out) != 8 {
		t.Fatalf("upsample length got %d", len(out))
	}
}

func TestResampleLinearEnds(t *testing.T) {
	in := []float32{0, 10}
	out := resampleLinear(in, 1000, 2000)
	if out[0] != 0 || out[len(out)-1] != 10 {
		t.Fatalf("endpoints not preserved: %v", out)
	}
}

func TestReadWAV16kMonoResamples(t *testing.T) {
	tmp := t.TempDir() + "/test.wav"
	sr := 8000
	ch := 2
	// 4 frames stereo: L,R pairs (non-cancelling)
	data := []int{1000, 900, 2000, 1900, 3000, 2900, 4000, 3900}
	buf := &audio.IntBuffer{
		Data:           data,
		Format:         &audio.Format{SampleRate: sr, NumChannels: ch},
		SourceBitDepth: 16,
	}
	f, err := os.Create(tmp)
	if err != nil {
		t.Fatal(err)
	}
	enc := wav.NewEncoder(f, sr, 16, ch, 1)
	if err := enc.Write(buf); err != nil {
		t.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := readWAV16kMono(tmp)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(out) != len(data)/ch*2 { // upsample 8k -> 16k doubles frames
		t.Fatalf("expected %d samples got %d", len(data)/ch*2, len(out))
	}
	// Ensure non-zero energy somewhere
	max := float32(0)
	for _, v := range out {
		if v < 0 {
			v = -v
		}
		if v > max {
			max = v
		}
	}
	if max == 0 {
		t.Fatalf("expected some energy after resample, got all zeros")
	}
}
