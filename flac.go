// Copyright 2015, David Howden
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tag

import (
	"errors"
	"io"
)

// blockType is a type which represents an enumeration of valid FLAC blocks
type blockType byte

// FLAC block types.
const (
	streamInfoBlock    blockType = 0
	// Padding Block               1
	// Application Block           2
	// Seektable Block             3
	// Cue Sheet Block             5
	vorbisCommentBlock blockType = 4
	pictureBlock       blockType = 6
)

// ReadFLACTags reads FLAC metadata from the io.ReadSeeker, returning the resulting
// metadata in a Metadata implementation, or non-nil error if there was a problem.
func ReadFLACTags(r io.ReadSeeker) (Metadata, error) {
	flac, err := readString(r, 4)
	if err != nil {
		return nil, err
	}
	if flac != "fLaC" {
		return nil, errors.New("expected 'fLaC'")
	}

	m := &metadataFLAC{
		newMetadataVorbis(), nil,
	}

	for {
		last, err := m.readFLACMetadataBlock(r)
		if err != nil {
			return nil, err
		}

		if last {
			break
		}
	}
	return m, nil
}

type metadataFLAC struct {
	*metadataVorbis

	flacmd5 []byte
}

func (m *metadataFLAC) readFLACMetadataBlock(r io.ReadSeeker) (last bool, err error) {
	blockHeader, err := readBytes(r, 1)
	if err != nil {
		return
	}

	if getBit(blockHeader[0], 7) {
		blockHeader[0] ^= (1 << 7)
		last = true
	}

	blockLen, err := readInt(r, 3)
	if err != nil {
		return
	}

	switch blockType(blockHeader[0]) {
	case streamInfoBlock:
		err = m.readStreamInfoBlock(r)

	case vorbisCommentBlock:
		err = m.readVorbisComment(r)

	case pictureBlock:
		err = m.readPictureBlock(r)

	default:
		_, err = r.Seek(int64(blockLen), io.SeekCurrent)
	}
	return
}

func (m *metadataFLAC) readStreamInfoBlock(r io.ReadSeeker) error {
	// skip 10 bytes
	_, err := r.Seek(10, io.SeekCurrent);
	if err != nil {
		return err
	}

	// FLAC encodes non-Vorbis comments as Big Endian
	streamInfo, err := readUint32BigEndian(r)
	streamInfo2, err := readUint32BigEndian(r)

	m.sampleRate		= uint(streamInfo >> 12)
	m.channels		= uint((streamInfo >> 9) & 0x7) + 1
	m.bitDepth		= uint((streamInfo >> 4) & 0x1f) + 1
	m.samples		= uint64(streamInfo & 0xf) << 32 + uint64(streamInfo2) 

	m.flacmd5, err = readBytes(r, 16)

	return nil
}

func (m *metadataFLAC) FileType() FileType {
	return FLAC
}

func (m *metadataFLAC) SampleRate() uint {
	return m.sampleRate
}

func (m *metadataFLAC) Channels() uint {
	return m.channels
}

func (m *metadataFLAC) BitDepth() uint {
	return m.bitDepth
}

func (m *metadataFLAC) Duration() uint {
	if m.sampleRate == 0 {
		return 0
	}
	return uint(m.samples / uint64(m.sampleRate))
}

func FLACMD5Sum(m *metadataFLAC) []byte {
	return m.flacmd5
}
