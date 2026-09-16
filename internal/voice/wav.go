package voice

import (
	"encoding/binary"
	"time"
)

// PCM16ToWAV — mono 16-bit PCM ni WAV fayliga o'raydi.
func PCM16ToWAV(pcm []byte, sampleRate int) []byte {
	buf := make([]byte, 44+len(pcm))
	copy(buf[0:], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(36+len(pcm))) //nolint:gosec // G115: audio hajmi MaxClipBytes bilan cheklangan
	copy(buf[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16)
	binary.LittleEndian.PutUint16(buf[20:], 1)                    // PCM
	binary.LittleEndian.PutUint16(buf[22:], 1)                    // mono
	binary.LittleEndian.PutUint32(buf[24:], uint32(sampleRate))   //nolint:gosec // G115: 8000..48000
	binary.LittleEndian.PutUint32(buf[28:], uint32(sampleRate*2)) //nolint:gosec // G115: 8000..48000
	binary.LittleEndian.PutUint16(buf[32:], 2)
	binary.LittleEndian.PutUint16(buf[34:], 16)
	copy(buf[36:], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(len(pcm))) //nolint:gosec // G115: yuqoridagi bilan bir xil
	copy(buf[44:], pcm)
	return buf
}

// TrimSilence — boshidagi va oxiridagi sukunatni kesadi ([pad] qoldiriladi).
//
// TTS iboraning oldi-ortiga yarim soniyagacha jimlik qo'yadi: "Hozir o'ngga
// buriling" kechikib eshitilardi va ketma-ket ikki ibora orasida uzilish bo'lardi.
func TrimSilence(pcm []byte, sampleRate int, threshold int32, pad time.Duration) []byte {
	n := len(pcm) / 2
	loud := func(i int) bool {
		v := int32(int16(binary.LittleEndian.Uint16(pcm[2*i:]))) //nolint:gosec // G115: 16-bit namuna
		if v < 0 {
			v = -v
		}
		return v >= threshold
	}
	first := -1
	for i := 0; i < n; i++ {
		if loud(i) {
			first = i
			break
		}
	}
	if first < 0 {
		return pcm[:0]
	}
	last := first
	for i := n - 1; i >= first; i-- {
		if loud(i) {
			last = i
			break
		}
	}
	padSamples := int(pad.Seconds() * float64(sampleRate))
	start := max(0, first-padSamples)
	end := min(n, last+1+padSamples)
	return pcm[2*start : 2*end]
}
