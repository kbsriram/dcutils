package mov

import (
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"os"
)

var ErrNotImplemented = errors.New("Sorry, not implemented this size")

// The Visit method is invoked for each Atom encountered by VisitAtoms.
type Visitor interface {
	Visit([]string, *io.SectionReader) error
}

type ReadAtSeeker interface {
	io.ReaderAt
	io.Seeker
}

// The VisitorFunc type is an adapter to allow using simple functions
// as Visitors. If f is a function with the appropriate signature,
// VisitorFunc(f) is a Visitor object that calls f.
type VisitorFunc func([]string, *io.SectionReader) error

func (f VisitorFunc) Visit(path []string, sr *io.SectionReader) error {
	return f(path, sr)
}

// VisitAtoms does a DFS visit of atoms in the provided mov file.
func VisitAtoms(v Visitor, rs ReadAtSeeker) error {
	len, err := seekEnd(rs)
	if err != nil {
		return err
	}
	return visitAtomList(make([]string, 0), v, io.NewSectionReader(rs, 0, len))
}

func visitAtomList(root []string, v Visitor, sr *io.SectionReader) error {
	for {
		ctype, csr, err := nextAtom(sr)
		if err != nil {
			if err == io.EOF {
				return nil
			} else {
				return err
			}
		}
		err = v.Visit(append(root, ctype), csr)
		if err != nil {
			return err
		}

		switch ctype {
		case "moov", "trak", "mdia", "minf", "stbl", "dinf":
			err = visitAtomList(append(root, ctype), v, csr)
			if err != nil {
				return err
			}
		}
	}
}

func nextAtom(sr *io.SectionReader) (string, *io.SectionReader, error) {
	var asz uint32
	var sz int64
	atyp := make([]byte, 4)
	if err := binary.Read(sr, binary.BigEndian, &asz); err != nil {
		return "", nil, err
	}
	if asz == 0 {
		// Size is entire section
		sz = sr.Size()
	} else if asz == 1 {
		return "", nil, ErrNotImplemented
	} else {
		sz = int64(asz)
	}
	if _, err := io.ReadFull(sr, atyp); err != nil {
		return "", nil, err
	}
	sz = sz - 8 // 4 bytes for size, 4 bytes for type

	// Get current offset
	cur, err := seekCur(sr)
	if err != nil {
		return "", nil, err
	}
	// Consume remainder of parent
	if _, err := io.CopyN(ioutil.Discard, sr, sz); err != nil {
		return "", nil, err
	}
	return string(atyp), io.NewSectionReader(sr, cur, sz), nil
}

func seekCur(s io.Seeker) (int64, error) {
	return s.Seek(0, os.SEEK_CUR)
}

func seekEnd(s io.Seeker) (int64, error) {
	return s.Seek(0, os.SEEK_END)
}
