// Copyright 2015, David Howden
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tag

import (
	"errors"
	"io"
)

// ReadDSFTags reads DSF metadata from the io.ReadSeeker, returning the resulting
// metadata in a Metadata implementation, or non-nil error if there was a problem.
// samples: http://www.2l.no/hires/index.html
func ReadDSFTags(r io.ReadSeeker) (Metadata, error) {
	dsd, err := readString(r, 4)
	if err != nil {
		return nil, err
	}
	if dsd != "DSD " {
		return nil, errors.New("expected 'DSD '")
	}

	_, err = r.Seek(int64(16), io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	n4, err := readBytes(r, 8)
	if err != nil {
		return nil, err
	}
	id3Pointer := getUintLittleEndian(n4)

	f, err := readString(r, 4)
	if err != nil {
		return nil, err
	}
	if f != "fmt " {
		return nil, errors.New("expected 'fmt '")
	}

	n4, err = readBytes(r, 8)
	if err != nil {
		return nil, err
	}
	fsize := getIntLittleEndian(n4)

	if fsize != 52 {
		return nil, errors.New("fmt section not 52 bytes long")
	}

	fmtVersion, err := readUint32LittleEndian(r)
	if err != nil {
		return nil, err
	}

	if fmtVersion != 1 {
		return nil, errors.New("fmt version != 1")
	}

	// skip Format ID and Channel Type
	_, err = r.Seek(8, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	v, err := readUint32LittleEndian(r)
	if err != nil {
		return nil, err
	}
	channels := uint(v)

	v, err = readUint32LittleEndian(r)
	if err != nil {
		return nil, err
	}
	sampleRate := uint(v)

	v, err = readUint32LittleEndian(r)
	if err != nil {
		return nil, err
	}
	bitDepth := uint(v)

	samplesLW, err := readUint32LittleEndian(r)
	if err != nil {
		return nil, err
	}

	samplesHW, err := readUint32LittleEndian(r)
	if err != nil {
		return nil, err
	}

	samples := uint64(samplesHW << 32 + samplesLW)
	_, err = r.Seek(int64(id3Pointer), io.SeekStart)
	if err != nil {
		return nil, err
	}

	id3, err := ReadID3v2Tags(r)
	if err != nil {
		return nil, err
	}

	return metadataDSF{sampleRate, channels, bitDepth, samples, id3}, nil
}

type metadataDSF struct {
	// audio data
	sampleRate uint
	channels uint
	bitDepth uint
	samples uint64

	id3 Metadata
}

func (m metadataDSF) Format() Format {
	return m.id3.Format()
}

func (m metadataDSF) FileType() FileType {
	return DSF
}

func (m metadataDSF) Title() string {
	return m.id3.Title()
}

func (m metadataDSF) Album() string {
	return m.id3.Album()
}

func (m metadataDSF) Artist() string {
	return m.id3.Artist()
}

func (m metadataDSF) AlbumArtist() string {
	return m.id3.AlbumArtist()
}

func (m metadataDSF) Composer() string {
	return m.id3.Composer()
}

func (m metadataDSF) Year() int {
	return m.id3.Year()
}

func (m metadataDSF) Genre() string {
	return m.id3.Genre()
}

func (m metadataDSF) Track() (int, int) {
	return m.id3.Track()
}

func (m metadataDSF) Disc() (int, int) {
	return m.id3.Disc()
}

func (m metadataDSF) Picture() *Picture {
	return m.id3.Picture()
}

func (m metadataDSF) Lyrics() string {
	return m.id3.Lyrics()
}

func (m metadataDSF) Comment() string {
	return m.id3.Comment()
}

func (m metadataDSF) SampleRate() uint {
	return m.sampleRate
}

func (m metadataDSF) Channels() uint {
	return m.channels
}

func (m metadataDSF) BitDepth() uint {
	return m.bitDepth
}

func (m metadataDSF) Duration() uint {
	if m.sampleRate == 0 {
		return 0
	}
	return uint(m.samples / uint64(m.sampleRate))
}

func (m metadataDSF) FLACMD5Sum() *[8]byte {
	return nil
}

func (m metadataDSF) Raw() map[string]interface{} {
	return m.id3.Raw()
}
