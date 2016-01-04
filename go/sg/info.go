// Copyright 2016 KB Sriram
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sg

import (
	"encoding/binary"
	"errors"
	"github.com/kbsriram/dcutils/go/mov"
	"io"
	"io/ioutil"
)

type GPSLog struct {
	Magic         [4]byte
	_             [36]byte
	Hour          uint32
	Min           uint32
	Sec           uint32
	Year          uint32
	Mon           uint32
	Day           uint32
	Unknown       byte
	LatitudeSpec  byte
	LongitudeSpec byte
	_             byte
	Latitude      float32
	Longitude     float32
	Speed         float32
	Bearing       float32
}

type SGInfo interface {
	GPSLogs() ([]GPSLog, error)
}

type sgInfo struct {
	ras mov.ReadAtSeeker
}

func NewInfo(ras mov.ReadAtSeeker) SGInfo {
	return &sgInfo{ras}
}

func (sgi *sgInfo) GPSLogs() ([]GPSLog, error) {

	sa := &sampleAccumulator{}
	if err := mov.VisitAtoms(sa, sgi.ras); err != nil {
		return nil, err
	}
	gpsLogs := make([]GPSLog, len(sa.audioOffsets))

	// Read the gps atom 64k bytes following each offset, and fail
	// if it doesn't exist.
	for i, offset := range sa.audioOffsets {
		goff := int64(offset + 0x10000)
		err := mov.VisitAtoms(accumulatorVisitor(&gpsLogs[i]),
			io.NewSectionReader(sgi.ras, goff, 0x8000))
		if err != nil {
			return nil, err
		}
	}
	return gpsLogs[:], nil
}

func accumulatorVisitor(gpsLog *GPSLog) mov.VisitorFunc {
	return func(path []string, sr *io.SectionReader) error {
		if err := binary.Read(sr, binary.LittleEndian, gpsLog); err != nil {
			return err
		}
		if string(gpsLog.Magic[:]) != "GPS " {
			return ErrInvalidGPS
		}
		return nil
	}
}

var ErrInvalidGPS = errors.New("Not a GPS block")

type sampleAccumulator struct {
	inSound      bool
	audioOffsets []uint32
}

func getAudioChunks(sr *io.SectionReader) ([]uint32, error) {
	// discard 4 bytes
	_, err := io.CopyN(ioutil.Discard, sr, 4)
	if err != nil {
		return nil, err
	}
	var nent uint32
	err = binary.Read(sr, binary.BigEndian, &nent)
	if err != nil {
		return nil, err
	}
	result := make([]uint32, nent)

	for i := range result {
		err = binary.Read(sr, binary.BigEndian, &result[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (sa *sampleAccumulator) Visit(path []string, sr *io.SectionReader) error {
	cur := path[len(path)-1]
	switch {
	case len(path) == 5 && cur == "smhd":
		sa.inSound = true
		return nil
	case len(path) < 5 || cur == "vmhd":
		sa.inSound = false
		return nil
	case sa.inSound && len(path) == 6 && path[len(path)-1] == "stco":
		offsets, err := getAudioChunks(sr)
		sa.audioOffsets = offsets
		return err
	default:
		return nil
	}
}
