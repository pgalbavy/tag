// Copyright 2015, David Howden
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tag

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

func newMetadataVorbis() *metadataVorbis {
	return &metadataVorbis{
		sampleRate: 0,
		channels: 0,
		bitDepth: 0,
		samples: 0,
		c: make(map[string]string),
	}
}

type metadataVorbis struct {
	// audio data
	sampleRate uint
	channels uint
	bitDepth uint
	samples uint64

	c map[string]string // the vorbis comments
	p *Picture
}

func (m *metadataVorbis) readVorbisComment(r io.Reader) error {
	vendorLen, err := readUint32LittleEndian(r)
	if err != nil {
		return err
	}

	vendor, err := readString(r, uint(vendorLen))
	if err != nil {
		return err
	}
	m.c["vendor"] = vendor

	commentsLen, err := readUint32LittleEndian(r)
	if err != nil {
		return err
	}

	for i := uint32(0); i < commentsLen; i++ {
		l, err := readUint32LittleEndian(r)
		if err != nil {
			return err
		}
		s, err := readString(r, uint(l))
		if err != nil {
			return err
		}
		k, v, err := parseComment(s)
		if err != nil {
			return err
		}
		if _, ok := m.c[strings.ToLower(k)]; ok {
			m.c[strings.ToLower(k)] = m.c[strings.ToLower(k)] + "\\\\" + v
		} else {
			m.c[strings.ToLower(k)] = v
		}
	}

	if b64data, ok := m.c["metadata_block_picture"]; ok {
		data, err := base64.StdEncoding.DecodeString(b64data)
		if err != nil {
			return err
		}
		m.readPictureBlock(bytes.NewReader(data))
	}

	return nil
}

func (m *metadataVorbis) readPictureBlock(r io.Reader) error {
	b, err := readInt(r, 4)
	if err != nil {
		return err
	}
	pictureType, ok := pictureTypes[byte(b)]
	if !ok {
		return fmt.Errorf("invalid picture type: %v", b)
	}
	mimeLen, err := readUint(r, 4)
	if err != nil {
		return err
	}
	mime, err := readString(r, mimeLen)
	if err != nil {
		return err
	}

	ext := ""
	switch mime {
	case "image/jpeg":
		ext = "jpg"
	case "image/png":
		ext = "png"
	case "image/gif":
		ext = "gif"
	}

	descLen, err := readUint(r, 4)
	if err != nil {
		return err
	}
	desc, err := readString(r, descLen)
	if err != nil {
		return err
	}

	// We skip width <32>, height <32>, colorDepth <32>, coloresUsed <32>
	_, err = readInt(r, 4) // width
	if err != nil {
		return err
	}
	_, err = readInt(r, 4) // height
	if err != nil {
		return err
	}
	_, err = readInt(r, 4) // color depth
	if err != nil {
		return err
	}
	_, err = readInt(r, 4) // colors used
	if err != nil {
		return err
	}

	dataLen, err := readInt(r, 4)
	if err != nil {
		return err
	}
	data := make([]byte, dataLen)
	_, err = io.ReadFull(r, data)
	if err != nil {
		return err
	}

	m.p = &Picture{
		Ext:         ext,
		MIMEType:    mime,
		Type:        pictureType,
		Description: desc,
		Data:        data,
	}
	return nil
}

func parseComment(c string) (k, v string, err error) {
	kv := strings.SplitN(c, "=", 2)
	if len(kv) != 2 {
		err = errors.New("vorbis comment must contain '='")
		return
	}
	k = kv[0]
	v = kv[1]
	return
}

func (m *metadataVorbis) Format() Format {
	return VORBIS
}

func (m *metadataVorbis) Raw() map[string]interface{} {
	raw := make(map[string]interface{}, len(m.c) + 4)
	if m.sampleRate > 0 {
		raw["_sampleRate"] = m.sampleRate
	}
	if m.samples > 0 {
		raw["_samples"] = m.samples
	}
	if m.channels > 0 {
		raw["_channels"] = m.channels
	}
	if m.bitDepth > 0 {
		raw["_bitdepth"] = m.bitDepth
	}
	for k, v := range m.c {
		raw[k] = v
	}
	return raw
}

func (m *metadataVorbis) Title() string {
	return m.c["title"]
}

func (m *metadataVorbis) Artist() string {
	// PERFORMER
	// The artist(s) who performed the work. In classical music this would be the
	// conductor, orchestra, soloists. In an audio book it would be the actor who
	// did the reading. In popular music this is typically the same as the ARTIST
	// and is omitted.
	if m.c["performer"] != "" {
		return m.c["performer"]
	}
	return m.c["artist"]
}

func (m *metadataVorbis) Album() string {
	return m.c["album"]
}

func (m *metadataVorbis) AlbumArtist() string {
	// This field isn't actually included in the standard, though
	// it is commonly assigned to albumartist.
	return m.c["albumartist"]
}

func (m *metadataVorbis) Composer() string {
	// ARTIST
	// The artist generally considered responsible for the work. In popular music
	// this is usually the performing band or singer. For classical music it would
	// be the composer. For an audio book it would be the author of the original text.
	if m.c["composer"] != "" {
		return m.c["composer"]
	}
	if m.c["performer"] == "" {
		return ""
	}
	return m.c["artist"]
}

func (m *metadataVorbis) Genre() string {
	return m.c["genre"]
}

func (m *metadataVorbis) Year() int {
	var dateFormat string

	// The date need to follow the international standard https://en.wikipedia.org/wiki/ISO_8601
	// and obviously the VorbisComment standard https://wiki.xiph.org/VorbisComment#Date_and_time
	switch len(m.c["date"]) {
	case 0:
		return 0
	case 4:
		dateFormat = "2006"
	case 7:
		dateFormat = "2006-01"
	case 10:
		dateFormat = "2006-01-02"
	}

	t, _ := time.Parse(dateFormat, m.c["date"])
	return t.Year()
}

func (m *metadataVorbis) Track() (int, int) {
	x, _ := strconv.Atoi(m.c["tracknumber"])
	// https://wiki.xiph.org/Field_names
	n, _ := strconv.Atoi(m.c["tracktotal"])
	return x, n
}

func (m *metadataVorbis) Disc() (int, int) {
	// https://wiki.xiph.org/Field_names
	x, _ := strconv.Atoi(m.c["discnumber"])
	n, _ := strconv.Atoi(m.c["disctotal"])
	return x, n
}

func (m *metadataVorbis) Lyrics() string {
	return m.c["lyrics"]
}

func (m *metadataVorbis) Comment() string {
	if m.c["comment"] != "" {
		return m.c["comment"]
	}
	return m.c["description"]
}

func (m *metadataVorbis) Picture() *Picture {
	return m.p
}

func (m *metadataVorbis) SampleRate() uint {
	return m.sampleRate
}

func (m *metadataVorbis) Channels() uint {
	return m.channels
}

func (m *metadataVorbis) BitDepth() uint {
	return m.bitDepth
}

func (m *metadataVorbis) Duration() uint {
	if m.sampleRate == 0 {
		return 0
	}
	return uint(m.samples / uint64(m.sampleRate))
}

func (m *metadataVorbis) FLACMD5Sum() *[8]byte {
	return nil
}
