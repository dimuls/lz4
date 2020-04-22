//+build go1.9

package lz4block_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/pierrec/lz4"
	"github.com/pierrec/lz4/internal/lz4block"
)

type testcase struct {
	file         string
	compressible bool
	src          []byte
}

var rawFiles = []testcase{
	// {"testdata/207326ba-36f8-11e7-954a-aca46ba8ca73.png", true, nil},
	{"../../testdata/e.txt", false, nil},
	{"../../testdata/gettysburg.txt", true, nil},
	{"../../testdata/Mark.Twain-Tom.Sawyer.txt", true, nil},
	{"../../testdata/pg1661.txt", true, nil},
	{"../../testdata/pi.txt", false, nil},
	{"../../testdata/random.data", false, nil},
	{"../../testdata/repeat.txt", true, nil},
	{"../../testdata/pg1661.txt", true, nil},
}

func TestCompressUncompressBlock(t *testing.T) {
	type compressor func(s, d []byte) (int, error)

	run := func(t *testing.T, tc testcase, compress compressor) int {
		t.Helper()
		src := tc.src

		// Compress the data.
		zbuf := make([]byte, lz4block.CompressBlockBound(len(src)))
		n, err := compress(src, zbuf)
		if err != nil {
			t.Error(err)
			return 0
		}
		zbuf = zbuf[:n]

		// Make sure that it was actually compressed unless not compressible.
		if !tc.compressible {
			return 0
		}

		if n == 0 || n >= len(src) {
			t.Errorf("data not compressed: %d/%d", n, len(src))
			return 0
		}

		// Uncompress the data.
		buf := make([]byte, len(src))
		n, err = lz4block.UncompressBlock(zbuf, buf)
		if err != nil {
			t.Fatal(err)
		} else if n < 0 || n > len(buf) {
			t.Fatalf("returned written bytes > len(buf): n=%d available=%d", n, len(buf))
		} else if n != len(src) {
			t.Errorf("expected to decompress into %d bytes got %d", len(src), n)
		}

		buf = buf[:n]
		if !bytes.Equal(src, buf) {
			var c int
			for i, b := range buf {
				if c > 10 {
					break
				}
				if src[i] != b {
					t.Errorf("%d: exp(%x) != got(%x)", i, src[i], buf[i])
					c++
				}
			}
			t.Fatal("uncompressed compressed data not matching initial input")
			return 0
		}

		return len(zbuf)
	}

	for _, tc := range rawFiles {
		src, err := ioutil.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		tc.src = src

		var n, nhc int
		t.Run("", func(t *testing.T) {
			tc := tc
			t.Run(tc.file, func(t *testing.T) {
				n = run(t, tc, func(src, dst []byte) (int, error) {
					return lz4block.CompressBlock(src, dst, nil)
				})
			})
			t.Run(fmt.Sprintf("%s HC", tc.file), func(t *testing.T) {
				nhc = run(t, tc, func(src, dst []byte) (int, error) {
					return lz4.CompressBlockHC(src, dst, 10, nil)
				})
			})
		})
		if !t.Failed() {
			t.Logf("%-40s: %8d / %8d / %8d\n", tc.file, n, nhc, len(src))
		}
	}
}

func TestCompressCornerCase_CopyDstUpperBound(t *testing.T) {
	type compressor func(s, d []byte) (int, error)

	run := func(src []byte, compress compressor) {
		t.Helper()

		// Compress the data.
		// We provide a destination that is too small to trigger an out-of-bounds,
		// which makes it return the error we want.
		zbuf := make([]byte, int(float64(len(src))*0.40))
		_, err := compress(src, zbuf)
		if err != lz4.ErrInvalidSourceShortBuffer {
			t.Fatal("err should be ErrInvalidSourceShortBuffer, was", err)
		}
	}

	file := "../../testdata/upperbound.data"
	src, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	t.Run(file, func(t *testing.T) {
		t.Parallel()
		run(src, func(src, dst []byte) (int, error) {
			return lz4block.CompressBlock(src, dst, nil)
		})
	})
	t.Run(fmt.Sprintf("%s HC", file), func(t *testing.T) {
		t.Parallel()
		run(src, func(src, dst []byte) (int, error) {
			return lz4block.CompressBlockHC(src, dst, 16)
		})
	})
}

func TestIssue23(t *testing.T) {
	compressBuf := make([]byte, lz4block.CompressBlockBound(1<<16))
	for j := 1; j < 16; j++ {
		var buf [1 << 16]byte

		for i := 0; i < len(buf); i += j {
			buf[i] = 1
		}

		n, _ := lz4block.CompressBlock(buf[:], compressBuf, nil)
		if got, want := n, 300; got > want {
			t.Fatalf("not able to compress repeated data: got %d; want %d", got, want)
		}
	}
}