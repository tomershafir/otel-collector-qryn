package compress

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/metrico/otel-collector/receiver/pyroscopereceiver/buf"
)

type codec uint8

const (
	Gzip codec = iota
)

// Decodes compressed streams
type Decompressor struct {
	uncompressedSizeBytes    int64
	maxUncompressedSizeBytes int64
	decoders                 map[codec]func(body io.Reader) (io.Reader, error)
}

// Creates a new decompressor
func NewDecompressor(uncompressedSizeBytes int64, maxUncompressedSizeBytes int64) *Decompressor {
	return &Decompressor{
		uncompressedSizeBytes:    uncompressedSizeBytes,
		maxUncompressedSizeBytes: maxUncompressedSizeBytes,
		decoders: map[codec]func(r io.Reader) (io.Reader, error){
			Gzip: func(r io.Reader) (io.Reader, error) {
				gr, err := gzip.NewReader(r)
				if err != nil {
					return nil, err
				}
				return gr, nil
			},
		},
	}
}

func (d *Decompressor) readBytes(r io.Reader) (*bytes.Buffer, error) {
	buf := buf.PrepareBuffer(d.uncompressedSizeBytes)

	// read max+1 to validate size via a single Read()
	lr := io.LimitReader(r, d.maxUncompressedSizeBytes+1)

	n, err := buf.ReadFrom(lr)
	if err != nil {
		return nil, err
	}
	if n < 1 {
		return nil, fmt.Errorf("empty profile")
	}
	if n > d.maxUncompressedSizeBytes {
		return nil, fmt.Errorf("body size exceeds the limit %d bytes", d.maxUncompressedSizeBytes)
	}
	return buf, nil
}

// Decodes the accepted reader, applying the configured size limit to avoid oom by compression bomb
func (d *Decompressor) Decompress(r io.Reader, c codec) (*bytes.Buffer, error) {
	decoder, ok := d.decoders[c]
	if !ok {
		return nil, fmt.Errorf("unsupported encoding")
	}

	dr, err := decoder(r)
	if err != nil {
		return nil, err
	}

	return d.readBytes(dr)
}